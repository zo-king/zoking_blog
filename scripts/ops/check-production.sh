#!/usr/bin/env bash
set -Eeuo pipefail

ROLE="app"
if [[ "${1:-}" == "--role" ]]; then
  ROLE="${2:-}"
fi

REPO_DIR="${REPO_DIR:-/opt/zoking-blog}"
ENV_FILE="${ENV_FILE:-${REPO_DIR}/infra/docker/.env.prod}"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_DIR}/infra/docker/compose.prod.yml}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/zoking-blog}"
DISK_WARN_PERCENT="${DISK_WARN_PERCENT:-80}"
BACKUP_MAX_AGE_HOURS="${BACKUP_MAX_AGE_HOURS:-36}"
WG_MAX_AGE_SECONDS="${WG_MAX_AGE_SECONDS:-300}"
ALERT_WEBHOOK_URL="${ALERT_WEBHOOK_URL:-}"

errors=()

record_error() {
  errors+=("$*")
  printf '[zoking-health] FAIL %s\n' "$*" >&2
}

pass() {
  printf '[zoking-health] PASS %s\n' "$*"
}

check_status() {
  local url="$1"
  local expected="$2"
  local status
  status="$(curl -fsS -o /dev/null -w '%{http_code}' --max-time 15 "$url" 2>/dev/null || true)"
  if [[ "$status" =~ $expected ]]; then
    pass "$url -> $status"
  else
    record_error "$url expected ${expected}, got ${status:-no-response}"
  fi
}

check_disk() {
  local path="$1"
  local used
  used="$(df -P "$path" | awk 'NR==2 {gsub(/%/, "", $5); print $5}')"
  if [[ "$used" =~ ^[0-9]+$ ]] && (( used < DISK_WARN_PERCENT )); then
    pass "disk ${path} ${used}% used"
  else
    record_error "disk ${path} is ${used:-unknown}% used; threshold ${DISK_WARN_PERCENT}%"
  fi
}

check_wireguard() {
  local latest now age
  latest="$(wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}' | sort -nr | head -n1)"
  now="$(date +%s)"
  if [[ "$latest" =~ ^[0-9]+$ ]] && (( latest > 0 )); then
    age=$((now - latest))
  else
    age=999999999
  fi
  if (( age <= WG_MAX_AGE_SECONDS )); then
    pass "WireGuard latest handshake ${age}s ago"
  else
    record_error "WireGuard latest handshake is ${age}s old"
  fi
}

check_service() {
  local service="$1"
  if systemctl is-active --quiet "$service"; then
    pass "systemd ${service} active"
  else
    record_error "systemd ${service} is not active"
  fi
}

check_failed_units() {
  local failed
  # A failed health-check unit remains in systemd's failed set until reset-failed.
  # Exclude the two probe units so a transient outage does not make every later
  # probe fail forever; real application, backup, and host failures remain visible.
  failed="$(systemctl --failed --no-legend --plain 2>/dev/null | awk '$1 != "zoking-healthcheck.service" && $1 != "zoking-edge-healthcheck.service" && NF {count++} END {print count+0}')"
  if [[ "$failed" == "0" ]]; then
    pass "no failed systemd units"
  else
    record_error "${failed} systemd unit(s) failed"
  fi
}

check_certificate() {
  local domain="$1"
  local not_after expiry now days
  not_after="$(timeout 20 openssl s_client -connect "${domain}:443" -servername "$domain" </dev/null 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2-)"
  if [[ -n "$not_after" ]]; then
    expiry="$(date -d "$not_after" +%s)"
    now="$(date +%s)"
    days=$(((expiry - now) / 86400))
    if (( days >= 14 )); then
      pass "TLS ${domain} expires in ${days}d"
    else
      record_error "TLS ${domain} expires in ${days}d"
    fi
  else
    record_error "could not read TLS certificate expiry for ${domain}"
  fi
}

check_app() {
  local compose=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
  local service container state health

  for service in postgres api worker admin site goatcounter; do
    container="$("${compose[@]}" ps -q "$service" 2>/dev/null || true)"
    if [[ -z "$container" ]]; then
      record_error "Compose service ${service} has no running container"
      continue
    fi
    state="$(docker inspect -f '{{ .State.Status }}' "$container" 2>/dev/null || true)"
    health="$(docker inspect -f '{{ if .State.Health }}{{ .State.Health.Status }}{{ else }}none{{ end }}' "$container" 2>/dev/null || true)"
    if [[ "$state" == "running" && "$health" != "unhealthy" ]]; then
      pass "Compose ${service} state=${state} health=${health}"
    else
      record_error "Compose ${service} state=${state:-unknown} health=${health:-unknown}"
    fi
  done

  check_status 'http://10.20.0.2:18080/readyz' '^200$'
  check_status 'http://10.20.0.2:1313/' '^200$'
  check_status 'http://10.20.0.2:8081/' '^200$'
  check_status 'http://10.20.0.2:8100/' '^(200|303)$'
  check_disk '/'
  check_wireguard
  check_failed_units

  local latest epoch age_hours
  latest="$(readlink -f "${BACKUP_ROOT}/daily/latest" 2>/dev/null || true)"
  if [[ -d "$latest" && -f "$latest/SHA256SUMS" ]]; then
    epoch="$(stat -c %Y "$latest")"
    age_hours=$((( $(date +%s) - epoch ) / 3600))
    if (( age_hours <= BACKUP_MAX_AGE_HOURS )); then
      pass "latest backup ${age_hours}h old"
    else
      record_error "latest backup is ${age_hours}h old"
    fi
  else
    record_error "latest backup link or manifest is missing"
  fi
}

check_edge() {
  check_service caddy
  check_service wg-quick@wg0
  check_status 'http://10.20.0.2:18080/readyz' '^200$'
  check_status 'http://10.20.0.2:1313/' '^200$'
  check_status 'http://10.20.0.2:8081/' '^200$'
  check_status 'http://10.20.0.2:8100/' '^(200|303)$'
  check_status 'https://zoking.tech/' '^200$'
  check_status 'https://api.zoking.tech/readyz' '^200$'
  check_status 'https://admin.zoking.tech/' '^200$'
  check_status 'https://preview.zoking.tech/' '^404$'
  check_status 'https://stats.zoking.tech/' '^(200|303)$'
  check_disk '/'
  check_wireguard
  check_failed_units

  local domain
  for domain in zoking.tech api.zoking.tech admin.zoking.tech preview.zoking.tech stats.zoking.tech; do
    check_certificate "$domain"
  done
}

send_alert() {
  local host message json
  host="$(hostname -f 2>/dev/null || hostname)"
  message="Zoking ${ROLE} health check failed on ${host}: $(IFS='; '; printf '%s' "${errors[*]}")"
  if [[ -n "$ALERT_WEBHOOK_URL" ]]; then
    json="$(printf '%s' "$message" | sed 's/\\/\\\\/g; s/"/\\"/g')"
    curl -fsS --max-time 15 -H 'Content-Type: application/json' -d "{\"text\":\"${json}\"}" "$ALERT_WEBHOOK_URL" >/dev/null || true
  fi
}

case "$ROLE" in
  app) check_app ;;
  edge) check_edge ;;
  *) printf 'unknown role: %s (expected app or edge)\n' "$ROLE" >&2; exit 2 ;;
esac

if ((${#errors[@]} > 0)); then
  send_alert
  exit 1
fi

printf '[zoking-health] all %s checks passed\n' "$ROLE"
