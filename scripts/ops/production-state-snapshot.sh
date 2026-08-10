#!/usr/bin/env bash
set -Eeuo pipefail

REPO_DIR="${REPO_DIR:-/opt/zoking-blog}"
ENV_FILE="${ENV_FILE:-${REPO_DIR}/infra/docker/.env.prod}"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_DIR}/infra/docker/compose.prod.yml}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/zoking-blog}"
OUTPUT_PATH="${STATE_SNAPSHOT_PATH:-/var/lib/zoking-ops/production-state.tsv}"
MODE="${1:---write}"

usage() {
  printf 'usage: %s [--write|--stdout]\n' "$0" >&2
  exit 2
}

fail() {
  printf '[zoking-state] ERROR: %s\n' "$*" >&2
  exit 1
}

[[ "$MODE" == "--write" || "$MODE" == "--stdout" ]] || usage
[[ -r "$COMPOSE_FILE" ]] || fail "Compose file is unavailable: $COMPOSE_FILE"
[[ -r "$ENV_FILE" ]] || fail "production env file is unavailable: $ENV_FILE"

if [[ "$MODE" == "--write" ]]; then
  [[ "$(id -u)" -eq 0 ]] || fail "--write must run as root"
  case "$OUTPUT_PATH" in
    /var/lib/zoking-ops/*) ;;
    *) fail "STATE_SNAPSHOT_PATH must stay below /var/lib/zoking-ops" ;;
  esac
  install -d -m 0750 "$(dirname "$OUTPUT_PATH")"
fi

sanitize() {
  local value="$1"
  value="${value//$'\r'/ }"
  value="${value//$'\n'/ }"
  value="${value//$'\t'/ }"
  printf '%s' "$value"
}

emit() {
  printf '%s\t%s\n' "$1" "$(sanitize "${2:-}")"
}

resolve_ipv4() {
  timeout 5s getent ahostsv4 "$1" 2>/dev/null | awk '{print $1}' | sort -u | paste -sd ',' - || true
}

http_status() {
  curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 3 --max-time 8 "$1" 2>/dev/null || true
}

compose=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
repo_git=(git -c "safe.directory=${REPO_DIR}" -C "$REPO_DIR")

snapshot() {
  local now host host_addresses repo_commit repo_branch tracked_status tracked_dirty
  local wg_address latest_handshake now_epoch age
  local latest_backup backup_stamp backup_epoch backup_age
  local service container image state health ports timer enabled active next last domain
  local ufw_output ufw_status ssh_allow_rules

  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  host="$(hostname -f 2>/dev/null || hostname)"
  emit schema_version 1
  emit generated_at_utc "$now"
  emit host "$host"
  host_addresses="$(ip -4 -o addr show scope global 2>/dev/null | awk '$2 !~ /^(docker|br-|veth)/ {print $2 "=" $4}' | sort -u | paste -sd ';' - || true)"
  emit host_ipv4_addresses "${host_addresses:-unknown}"
  repo_commit="$(GIT_OPTIONAL_LOCKS=0 "${repo_git[@]}" rev-parse HEAD 2>/dev/null || true)"
  repo_branch="$(GIT_OPTIONAL_LOCKS=0 "${repo_git[@]}" branch --show-current 2>/dev/null || true)"
  emit repo_commit "${repo_commit:-unknown}"
  emit repo_branch "${repo_branch:-detached}"
  if tracked_status="$(GIT_OPTIONAL_LOCKS=0 "${repo_git[@]}" status --porcelain --untracked-files=no 2>/dev/null)"; then
    tracked_dirty="$(printf '%s\n' "$tracked_status" | awk 'NF {count++} END {print count+0}')"
  else
    tracked_dirty="unknown"
  fi
  emit repo_tracked_dirty_count "$tracked_dirty"

  wg_address="$(ip -4 -o addr show dev wg0 scope global 2>/dev/null | awk '{print $4}' | head -n1 || true)"
  emit wireguard_address "${wg_address:-unknown}"
  latest_handshake="$(wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}' | sort -nr | head -n1 || true)"
  now_epoch="$(date +%s)"
  if [[ "$latest_handshake" =~ ^[0-9]+$ ]] && ((latest_handshake > 0)); then
    age=$((now_epoch - latest_handshake))
  else
    age="unknown"
  fi
  emit wireguard_latest_handshake_age_seconds "$age"

  ufw_output="$(ufw status numbered 2>/dev/null || true)"
  if [[ -n "$ufw_output" ]]; then
    ufw_status="$(printf '%s\n' "$ufw_output" | sed -n '1p')"
    ssh_allow_rules="$(printf '%s\n' "$ufw_output" | awk '
      /ALLOW IN/ {
        line=$0
        sub(/^\[[[:space:]]*[0-9]+\][[:space:]]*/, "", line)
        sub(/[[:space:]]+#.*/, "", line)
        if (line ~ /(^|[[:space:]])22(\/tcp)?([[:space:]]|$)/) {
          if (count > 0) printf ";"
          printf "%s", line
          count++
        }
      }
    ')"
    emit firewall.ufw_status "${ufw_status:-unknown}"
    emit firewall.ssh_allow_rules "${ssh_allow_rules:-none}"
  else
    emit firewall.ufw_status unavailable
    emit firewall.ssh_allow_rules unavailable
  fi

  for service in postgres api worker admin site goatcounter; do
    container="$("${compose[@]}" ps -q "$service" 2>/dev/null | head -n1 || true)"
    if [[ -z "$container" ]]; then
      emit "service.${service}.state" missing
      continue
    fi
    image="$(docker inspect -f '{{.Image}}' "$container" 2>/dev/null || true)"
    state="$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null || true)"
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
    ports="$(docker port "$container" 2>/dev/null | paste -sd ';' - || true)"
    emit "service.${service}.image_digest" "$image"
    emit "service.${service}.state" "$state"
    emit "service.${service}.health" "$health"
    emit "service.${service}.ports" "$ports"
  done

  latest_backup="$(readlink -f "${BACKUP_ROOT}/daily/latest" 2>/dev/null || true)"
  if [[ -d "$latest_backup" ]]; then
    backup_stamp="$(basename "$latest_backup")"
    backup_epoch="$(stat -c %Y "$latest_backup" 2>/dev/null || true)"
    if [[ "$backup_epoch" =~ ^[0-9]+$ ]]; then
      backup_age=$(((now_epoch - backup_epoch) / 3600))
    else
      backup_age="unknown"
    fi
    emit backup_latest_stamp "$backup_stamp"
    emit backup_latest_age_hours "$backup_age"
    emit backup_latest_manifest "$([[ -f "$latest_backup/SHA256SUMS" ]] && echo present || echo missing)"
  else
    emit backup_latest_stamp missing
    emit backup_latest_age_hours unknown
    emit backup_latest_manifest missing
  fi

  emit disk_root_percent "$(df -P / | awk 'NR == 2 {gsub(/%/, "", $5); print $5}')"
  emit disk_backup_percent "$(df -P "$BACKUP_ROOT" 2>/dev/null | awk 'NR == 2 {gsub(/%/, "", $5); print $5}' || true)"

  for timer in zoking-backup.timer zoking-healthcheck.timer zoking-production-state-snapshot.timer; do
    enabled="$(systemctl is-enabled "$timer" 2>/dev/null || true)"
    active="$(systemctl is-active "$timer" 2>/dev/null || true)"
    next="$(systemctl show "$timer" -p NextElapseUSecRealtime --value 2>/dev/null || true)"
    last="$(systemctl show "$timer" -p LastTriggerUSec --value 2>/dev/null || true)"
    emit "timer.${timer}.enabled" "$enabled"
    emit "timer.${timer}.active" "$active"
    emit "timer.${timer}.next" "$next"
    emit "timer.${timer}.last" "$last"
  done

  for domain in zoking.tech api.zoking.tech admin.zoking.tech preview.zoking.tech stats.zoking.tech; do
    emit "dns.${domain}.ipv4" "$(resolve_ipv4 "$domain")"
  done

  emit endpoint.local_api_ready "$(http_status http://10.20.0.2:18080/readyz)"
  emit endpoint.local_site "$(http_status http://10.20.0.2:1313/)"
  emit endpoint.local_admin "$(http_status http://10.20.0.2:8081/)"
  emit endpoint.local_stats "$(http_status http://10.20.0.2:8100/)"
  emit endpoint.public_site "$(http_status https://zoking.tech/)"
  emit endpoint.public_api "$(http_status https://api.zoking.tech/readyz)"
  emit endpoint.public_admin "$(http_status https://admin.zoking.tech/)"
  emit endpoint.public_preview_root "$(http_status https://preview.zoking.tech/)"
  emit endpoint.public_stats "$(http_status https://stats.zoking.tech/)"
}

if [[ "$MODE" == "--stdout" ]]; then
  snapshot
  exit 0
fi

tmp_path="$(mktemp "${OUTPUT_PATH}.tmp.XXXXXX")"
trap 'rm -f -- "$tmp_path"' EXIT
snapshot > "$tmp_path"
chown root:root "$tmp_path"
chmod 0640 "$tmp_path"
mv -f -- "$tmp_path" "$OUTPUT_PATH"
trap - EXIT
printf '[zoking-state] wrote %s\n' "$OUTPUT_PATH"
