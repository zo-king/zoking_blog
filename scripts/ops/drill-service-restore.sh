#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

REPO_DIR="${REPO_DIR:-/opt/zoking-blog}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/zoking-blog/daily/latest}"
IDENTITY=""
ADMIN_ACCOUNT=""
API_PORT="${RESTORE_API_PORT:-28080}"
SITE_PORT="${RESTORE_SITE_PORT:-21313}"
PREFLIGHT_ONLY=false
SKIP_PUBLISH=false

POSTGRES_IMAGE="postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
SITE_IMAGE="nginx:1.30-alpine@sha256:97d490c12ba55b4946b01546d1c3ed324e8d41ab1c9fcb2a616aa470620e5b46"

usage() {
  cat <<'EOF'
Usage:
  sudo scripts/ops/drill-service-restore.sh --identity /run/user/<uid>/zoking-age-identity.txt [options]
  sudo scripts/ops/drill-service-restore.sh --preflight [--backup DIR]

Options:
  --backup DIR             Encrypted backup directory (default: daily/latest)
  --identity PATH          Temporary age identity under /run; deleted on exit
  --admin-account ACCOUNT  Restored administrator account (default: active super admin)
  --api-port PORT          Loopback API port (default: 28080)
  --site-port PORT         Loopback site port (default: 21313)
  --skip-publish           Verify login but do not exercise the isolated worker
  --preflight              Read-only dependency, backup, image, and port checks
  -h, --help               Show this help

The drill never connects to production volumes or the production database. It
uses an internal Docker network, dedicated temporary volumes, and loopback-only
ports. The temporary age identity is removed on every normal or error exit.
EOF
}

log() {
  printf '[zoking-restore] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

valid_port() {
  [[ "$1" =~ ^[0-9]+$ ]] && ((10#$1 >= 1024 && 10#$1 <= 65535))
}

while (($# > 0)); do
  case "$1" in
    --backup)
      (($# >= 2)) || fail "--backup requires a directory"
      BACKUP_DIR="$2"
      shift 2
      ;;
    --identity)
      (($# >= 2)) || fail "--identity requires a path"
      IDENTITY="$2"
      shift 2
      ;;
    --admin-account)
      (($# >= 2)) || fail "--admin-account requires a value"
      ADMIN_ACCOUNT="$2"
      shift 2
      ;;
    --api-port)
      (($# >= 2)) || fail "--api-port requires a value"
      API_PORT="$2"
      shift 2
      ;;
    --site-port)
      (($# >= 2)) || fail "--site-port requires a value"
      SITE_PORT="$2"
      shift 2
      ;;
    --skip-publish)
      SKIP_PUBLISH=true
      shift
      ;;
    --preflight)
      PREFLIGHT_ONLY=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || fail "run as root so the protected backup can be read"
valid_port "$API_PORT" || fail "API port must be between 1024 and 65535"
valid_port "$SITE_PORT" || fail "site port must be between 1024 and 65535"
[[ "$API_PORT" != "$SITE_PORT" ]] || fail "API and site ports must differ"

for command_name in age curl docker jq openssl readlink sha256sum ss tar; do
  require_command "$command_name"
done

REPO_DIR="$(readlink -f "$REPO_DIR")"
BACKUP_DIR="$(readlink -f "$BACKUP_DIR")"
VERIFY_SCRIPT="${REPO_DIR}/scripts/ops/verify-backup.sh"
COMPOSE_FILE="${REPO_DIR}/infra/docker/compose.prod.yml"
ENV_FILE="${REPO_DIR}/infra/docker/.env.prod"
SITE_CONFIG="${REPO_DIR}/infra/docker/site.nginx.conf"

[[ -d "$REPO_DIR/.git" ]] || fail "repository not found: $REPO_DIR"
[[ -x "$VERIFY_SCRIPT" ]] || fail "backup verifier is unavailable: $VERIFY_SCRIPT"
[[ -r "$COMPOSE_FILE" ]] || fail "production Compose file is unavailable"
[[ -r "$ENV_FILE" ]] || fail "production env file is unavailable"
[[ -r "$SITE_CONFIG" ]] || fail "site nginx config is unavailable"
case "$BACKUP_DIR" in
  /var/backups/zoking-blog/daily/*|/var/backups/zoking-blog/weekly/*|/var/backups/zoking-blog/monthly/*) ;;
  *) fail "backup must resolve below /var/backups/zoking-blog/{daily,weekly,monthly}"
esac
[[ -d "$BACKUP_DIR" ]] || fail "backup directory not found: $BACKUP_DIR"

COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
PRODUCTION_API_ID="$("${COMPOSE[@]}" ps -q api)"
[[ -n "$PRODUCTION_API_ID" ]] || fail "production API container is not running"
[[ "$(docker inspect -f '{{.State.Running}}' "$PRODUCTION_API_ID")" == "true" ]] || fail "production API container is not running"
API_IMAGE="$(docker inspect -f '{{.Image}}' "$PRODUCTION_API_ID")"
[[ "$API_IMAGE" =~ ^sha256:[0-9a-f]{64}$ ]] || fail "could not resolve immutable production API image"
PRODUCTION_PROJECT="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$PRODUCTION_API_ID")"
[[ -n "$PRODUCTION_PROJECT" ]] || fail "could not resolve production Compose project"
mapfile -t PRODUCTION_IDS < <(docker ps -q --filter "label=com.docker.compose.project=${PRODUCTION_PROJECT}" | sort)
(( ${#PRODUCTION_IDS[@]} > 0 )) || fail "production Compose containers are unavailable"

for port in "$API_PORT" "$SITE_PORT"; do
  if ss -H -ltn "sport = :${port}" | grep -q .; then
    fail "loopback drill port is already in use: ${port}"
  fi
done

"$VERIFY_SCRIPT" "$BACKUP_DIR"
docker image inspect "$API_IMAGE" "$POSTGRES_IMAGE" "$SITE_IMAGE" >/dev/null

if [[ "$PREFLIGHT_ONLY" == "true" ]]; then
  [[ -z "$IDENTITY" ]] || fail "--identity is not used with --preflight"
  log "preflight passed"
  log "backup=$(basename "$BACKUP_DIR") api_image=${API_IMAGE} loopback_ports=${API_PORT},${SITE_PORT}"
  exit 0
fi

[[ -n "$IDENTITY" ]] || fail "--identity is required outside preflight mode"
[[ -e "$IDENTITY" ]] || fail "age identity does not exist"
IDENTITY="$(readlink -f "$IDENTITY")"
SUDO_USER_ID="${SUDO_UID:-0}"
case "$IDENTITY" in
  "/run/user/${SUDO_USER_ID}/"*|/run/zoking-recovery/*) ;;
  *) fail "age identity must be a temporary file under /run/user/${SUDO_USER_ID}/ or /run/zoking-recovery/"
esac
[[ -f "$IDENTITY" && -r "$IDENTITY" ]] || fail "age identity is not a readable regular file"
if find "$IDENTITY" -maxdepth 0 -perm /077 -print -quit | grep -q .; then
  fail "age identity must not be accessible by group or other users"
fi

DRILL_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
PREFIX="zoking-restore-${DRILL_ID}"
NETWORK="${PREFIX}-net"
POSTGRES_CONTAINER="${PREFIX}-postgres"
API_CONTAINER="${PREFIX}-api"
WORKER_CONTAINER="${PREFIX}-worker"
SITE_CONTAINER="${PREFIX}-site"
POSTGRES_VOLUME="${PREFIX}-postgres"
MEDIA_VOLUME="${PREFIX}-media"
RELEASES_VOLUME="${PREFIX}-releases"
PUBLISHER_VOLUME="${PREFIX}-publisher"
WORK_DIR=""
DELETE_IDENTITY=true
START_EPOCH="$(date +%s)"
DRILL_SUCCEEDED=false
ADMIN_PASSWORD=""
ACCESS_TOKEN=""

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  ADMIN_PASSWORD=""
  ACCESS_TOKEN=""

  if [[ "$status" -ne 0 && -n "${API_CONTAINER:-}" ]]; then
    log "drill failed; recent isolated API/worker logs follow" >&2
    docker logs --tail 40 "$API_CONTAINER" 2>&1 || true
    docker logs --tail 80 "$WORKER_CONTAINER" 2>&1 || true
  fi

  docker rm -f "$SITE_CONTAINER" "$WORKER_CONTAINER" "$API_CONTAINER" "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$PUBLISHER_VOLUME" "$RELEASES_VOLUME" "$MEDIA_VOLUME" "$POSTGRES_VOLUME" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true

  if [[ -n "$WORK_DIR" ]]; then
    case "$WORK_DIR" in
      /var/tmp/zoking-service-restore.*) rm -rf -- "$WORK_DIR" ;;
      *) log "refusing to remove unexpected work directory: $WORK_DIR" >&2 ;;
    esac
  fi
  if [[ "$DELETE_IDENTITY" == "true" ]]; then
    case "$IDENTITY" in
      "/run/user/${SUDO_USER_ID}/"*|/run/zoking-recovery/*) rm -f -- "$IDENTITY" ;;
      *) log "refusing to remove unexpected identity path: $IDENTITY" >&2 ;;
    esac
  fi

  if (( ${#PRODUCTION_IDS[@]} > 0 )); then
    local current_ids
    current_ids="$(docker ps -q --filter "label=com.docker.compose.project=${PRODUCTION_PROJECT}" | sort)"
    local expected_ids
    expected_ids="$(printf '%s\n' "${PRODUCTION_IDS[@]}")"
    if [[ "$current_ids" != "$expected_ids" ]]; then
      log "WARNING: production container IDs changed during the drill" >&2
      status=1
    fi
  fi

  if [[ "$DRILL_SUCCEEDED" == "true" && "$status" -eq 0 ]]; then
    log "temporary containers, network, volumes, decrypted data, and age identity removed"
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

WORK_DIR="$(mktemp -d /var/tmp/zoking-service-restore.XXXXXX)"

log "verifying and decrypting backup $(basename "$BACKUP_DIR")"
BACKUP_AGE_IDENTITY="$IDENTITY" "$VERIFY_SCRIPT" "$BACKUP_DIR"
for artifact in postgres.dump media-data.tar.gz site-releases.tar.gz publisher-site.tar.gz git-commit.txt; do
  age --decrypt --identity "$IDENTITY" --output "${WORK_DIR}/${artifact}" "${BACKUP_DIR}/${artifact}.age"
done

BACKUP_COMMIT="$(tr -d '[:space:]' <"${WORK_DIR}/git-commit.txt")"
[[ "$BACKUP_COMMIT" =~ ^[0-9a-f]{40}$ ]] || fail "backup git commit is invalid"

docker network create --internal --label zoking.restore-drill=true "$NETWORK" >/dev/null
for volume in "$POSTGRES_VOLUME" "$MEDIA_VOLUME" "$RELEASES_VOLUME" "$PUBLISHER_VOLUME"; do
  docker volume create --label zoking.restore-drill=true "$volume" >/dev/null
done

restore_archive() {
  local archive="$1"
  local volume="$2"
  docker run --rm \
    --network none \
    --volume "${volume}:/restore" \
    --volume "${WORK_DIR}:/backup:ro" \
    "$SITE_IMAGE" \
    sh -ec "tar -xzf '/backup/${archive}' -C /restore"
}

restore_archive media-data.tar.gz "$MEDIA_VOLUME"
restore_archive site-releases.tar.gz "$RELEASES_VOLUME"
restore_archive publisher-site.tar.gz "$PUBLISHER_VOLUME"

POSTGRES_PASSWORD="$(openssl rand -hex 24)"
JWT_SECRET="$(openssl rand -hex 32)"
PRIVACY_SECRET="$(openssl rand -hex 32)"
DATABASE_URL="postgres://restore:${POSTGRES_PASSWORD}@postgres:5432/zoking_blog_restore?sslmode=disable"

docker run -d \
  --name "$POSTGRES_CONTAINER" \
  --label zoking.restore-drill=true \
  --network "$NETWORK" \
  --network-alias postgres \
  --env POSTGRES_DB=zoking_blog_restore \
  --env POSTGRES_USER=restore \
  --env "POSTGRES_PASSWORD=${POSTGRES_PASSWORD}" \
  --volume "${POSTGRES_VOLUME}:/var/lib/postgresql/data" \
  "$POSTGRES_IMAGE" >/dev/null

ready_count=0
for _ in $(seq 1 120); do
  if docker exec "$POSTGRES_CONTAINER" pg_isready -U restore -d zoking_blog_restore >/dev/null 2>&1; then
    ((ready_count += 1))
    ((ready_count >= 2)) && break
  else
    ready_count=0
  fi
  sleep 1
done
((ready_count >= 2)) || fail "isolated PostgreSQL did not become ready"

docker exec -i "$POSTGRES_CONTAINER" pg_restore \
  -U restore -d zoking_blog_restore \
  --exit-on-error --clean --if-exists --no-owner --no-privileges \
  <"${WORK_DIR}/postgres.dump"

COMMON_ENV=(
  --env APP_ENV=recovery
  --env APP_PORT=18080
  --env "DATABASE_URL=${DATABASE_URL}"
  --env "JWT_SECRET=${JWT_SECRET}"
  --env "PRIVACY_HASH_SECRET=${PRIVACY_SECRET}"
  --env SITE_BASE_URL=https://restore.invalid/
  --env PUBLIC_API_BASE_URL=https://api.restore.invalid
  --env CORS_ALLOWED_ORIGINS=https://restore.invalid
  --env ADMIN_ALLOWED_ORIGINS=https://admin.restore.invalid
  --env TRUSTED_PROXIES=127.0.0.1,::1
  --env MEDIA_STORAGE_DRIVER=local
  --env MEDIA_LOCAL_DIR=/data/media
  --env MEDIA_PUBLIC_BASE_URL=/media-files
  --env HUGO_SITE_DIR=/workspace/apps/site
  --env HUGO_PUBLIC_DIR=/data/current
  --env HUGO_BIN=/usr/local/bin/hugo
  --env PAGEFIND_BIN=/usr/local/bin/pagefind
  --env MIGRATIONS_DIR=/workspace/db/migrations
  --env PUBLISH_RELEASE_ROOT=/data/releases
  --env PUBLISH_CURRENT_DIR=/data/current
  --env PUBLISH_PREVIEW_ROOT=/data/previews
  --env PUBLISH_PREVIEW_PUBLIC_BASE_URL=https://preview.restore.invalid/preview-files
  --env PUBLISH_WORKER_ENABLED=false
  --env HUGO_COMMENTS_API_BASE=https://api.restore.invalid
  --env HUGO_STATS_HOST=stats.restore.invalid
)
COMMON_VOLUMES=(
  --volume "${REPO_DIR}/db/migrations:/workspace/db/migrations:ro"
  --volume "${PUBLISHER_VOLUME}:/workspace/apps/site"
  --volume "${MEDIA_VOLUME}:/data/media"
  --volume "${RELEASES_VOLUME}:/data"
)

docker run --rm \
  --network "$NETWORK" \
  "${COMMON_ENV[@]}" \
  --volume "${REPO_DIR}/db/migrations:/workspace/db/migrations:ro" \
  "$API_IMAGE" /app/migrate up

docker run -d \
  --name "$API_CONTAINER" \
  --label zoking.restore-drill=true \
  --network "$NETWORK" \
  --publish "127.0.0.1:${API_PORT}:18080" \
  "${COMMON_ENV[@]}" \
  "${COMMON_VOLUMES[@]}" \
  "$API_IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if curl --fail --silent --show-error "http://127.0.0.1:${API_PORT}/readyz" >/dev/null; then
    break
  fi
  sleep 1
done
curl --fail --silent --show-error "http://127.0.0.1:${API_PORT}/readyz" >/dev/null || fail "restored API readiness failed"
curl --fail --silent --show-error "http://127.0.0.1:${API_PORT}/api/v1/public/posts?page=1&page_size=1" >/dev/null || fail "restored public API read failed"
curl --fail --silent --show-error "http://127.0.0.1:${API_PORT}/api/v1/public/site/public-settings" >/dev/null || fail "restored site settings read failed"

docker run -d \
  --name "$SITE_CONTAINER" \
  --label zoking.restore-drill=true \
  --network "$NETWORK" \
  --publish "127.0.0.1:${SITE_PORT}:80" \
  --volume "${RELEASES_VOLUME}:/data:ro" \
  --volume "${MEDIA_VOLUME}:/data/media:ro" \
  --volume "${SITE_CONFIG}:/etc/nginx/conf.d/default.conf:ro" \
  "$SITE_IMAGE" >/dev/null

for _ in $(seq 1 30); do
  if curl --fail --silent --show-error "http://127.0.0.1:${SITE_PORT}/" >/dev/null; then
    break
  fi
  sleep 1
done
curl --fail --silent --show-error "http://127.0.0.1:${SITE_PORT}/" >/dev/null || fail "restored site read failed"

MEDIA_COUNT="$(docker run --rm --network none --volume "${MEDIA_VOLUME}:/data:ro" "$SITE_IMAGE" sh -ec 'find /data -type f ! -path "/data/.zoking-private/*" | wc -l')"
if ((MEDIA_COUNT > 0)); then
  MEDIA_PATH="$(docker run --rm --network none --volume "${MEDIA_VOLUME}:/data:ro" "$SITE_IMAGE" sh -ec 'find /data -type f ! -path "/data/.zoking-private/*" | sed "s#^/data/##" | grep -E "^[A-Za-z0-9._/-]+$" | head -n 1')"
  if [[ -n "$MEDIA_PATH" ]]; then
    curl --fail --silent --show-error "http://127.0.0.1:${SITE_PORT}/media-files/${MEDIA_PATH}" >/dev/null || fail "restored media read failed"
  else
    log "media archive has files but no path safe for an automated URL probe"
  fi
else
  MEDIA_STATUS="$(curl --silent --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:${SITE_PORT}/media-files/__restore-probe-missing__")"
  [[ "$MEDIA_STATUS" == "404" ]] || fail "empty restored media route returned HTTP ${MEDIA_STATUS}, expected 404"
fi

if [[ -z "$ADMIN_ACCOUNT" ]]; then
  ADMIN_ACCOUNT="$(docker exec "$POSTGRES_CONTAINER" psql -U restore -d zoking_blog_restore -Atc \
    "select coalesce(nullif(u.username, ''), u.email) from users u join user_roles ur on ur.user_id=u.id join roles r on r.id=ur.role_id where u.status='active' and u.deleted_at is null and r.code='super_admin' order by u.created_at limit 1")"
fi
[[ -n "$ADMIN_ACCOUNT" ]] || fail "no active restored super administrator was found"
[[ -r /dev/tty && -w /dev/tty ]] || fail "administrator login verification requires an interactive terminal"

printf 'Restored admin password for %s: ' "$ADMIN_ACCOUNT" >/dev/tty
IFS= read -r -s ADMIN_PASSWORD </dev/tty
printf '\n' >/dev/tty
[[ -n "$ADMIN_PASSWORD" ]] || fail "administrator password was empty"

LOGIN_RESPONSE="${WORK_DIR}/login-response.json"
COOKIE_JAR="${WORK_DIR}/cookies.txt"
LOGIN_STATUS="$(
  jq -n --arg account "$ADMIN_ACCOUNT" --arg password "$ADMIN_PASSWORD" '{account:$account,password:$password}' |
    curl --silent --show-error \
      --output "$LOGIN_RESPONSE" \
      --write-out '%{http_code}' \
      --cookie-jar "$COOKIE_JAR" \
      --header 'Content-Type: application/json' \
      --header 'Origin: https://admin.restore.invalid' \
      --data-binary @- \
      "http://127.0.0.1:${API_PORT}/api/v1/admin/auth/login"
)"
ADMIN_PASSWORD=""
[[ "$LOGIN_STATUS" == "200" ]] || fail "restored administrator login returned HTTP ${LOGIN_STATUS}"
jq -e '.data.user.id and .data.csrf_token' "$LOGIN_RESPONSE" >/dev/null || fail "restored administrator login response was invalid"
ACCESS_TOKEN="$(awk '$6 == "zoking_admin_access" { print $7 }' "$COOKIE_JAR" | tail -n 1)"
[[ -n "$ACCESS_TOKEN" ]] || fail "restored administrator access token was not issued"
AUTH_CONFIG="${WORK_DIR}/curl-auth.conf"
printf 'header = "Authorization: Bearer %s"\n' "$ACCESS_TOKEN" >"$AUTH_CONFIG"

curl --fail --silent --show-error \
  --config "$AUTH_CONFIG" \
  --header 'Origin: https://admin.restore.invalid' \
  "http://127.0.0.1:${API_PORT}/api/v1/admin/auth/me" >/dev/null || fail "restored authenticated API read failed"

if [[ "$SKIP_PUBLISH" != "true" ]]; then
  docker run -d \
    --name "$WORKER_CONTAINER" \
    --label zoking.restore-drill=true \
    --network "$NETWORK" \
    "${COMMON_ENV[@]}" \
    "${COMMON_VOLUMES[@]}" \
    "$API_IMAGE" /app/worker >/dev/null

  sleep 2
  [[ "$(docker inspect -f '{{.State.Running}}' "$WORKER_CONTAINER")" == "true" ]] || fail "isolated worker did not remain running"

  PUBLISH_RESPONSE="${WORK_DIR}/publish-response.json"
  PUBLISH_STATUS="$(curl --silent --show-error \
    --output "$PUBLISH_RESPONSE" \
    --write-out '%{http_code}' \
    --config "$AUTH_CONFIG" \
    --header 'Origin: https://admin.restore.invalid' \
    --request POST \
    "http://127.0.0.1:${API_PORT}/api/v1/admin/settings/publish")"
  [[ "$PUBLISH_STATUS" == "202" ]] || fail "isolated publish request returned HTTP ${PUBLISH_STATUS}"
  JOB_ID="$(jq -er '.data.job.id' "$PUBLISH_RESPONSE")" || fail "isolated publish response did not contain a job ID"

  JOB_STATUS=""
  for _ in $(seq 1 180); do
    JOB_RESPONSE="$(curl --fail --silent --show-error --config "$AUTH_CONFIG" \
      --header 'Origin: https://admin.restore.invalid' \
      "http://127.0.0.1:${API_PORT}/api/v1/admin/publish/jobs/${JOB_ID}")"
    NEW_STATUS="$(jq -er '.data.status' <<<"$JOB_RESPONSE")" || fail "could not read isolated publish status"
    if [[ "$NEW_STATUS" != "$JOB_STATUS" ]]; then
      log "isolated publish status=${NEW_STATUS}"
      JOB_STATUS="$NEW_STATUS"
    fi
    case "$JOB_STATUS" in
      published) break ;;
      failed|canceled)
        ERROR_CODE="$(jq -r '.data.error_code // "unknown"' <<<"$JOB_RESPONSE")"
        fail "isolated publish ended with status=${JOB_STATUS} error_code=${ERROR_CODE}"
        ;;
    esac
    sleep 1
  done
  [[ "$JOB_STATUS" == "published" ]] || fail "isolated publish did not finish within 180 seconds"
  curl --fail --silent --show-error "http://127.0.0.1:${SITE_PORT}/" >/dev/null || fail "site read failed after isolated publish"
fi

COUNTS="$(docker exec "$POSTGRES_CONTAINER" psql -U restore -d zoking_blog_restore -AtF/ -c \
  "select (select count(*) from users),(select count(*) from posts),(select count(*) from media_assets),(select count(*) from publish_jobs)")"
END_EPOCH="$(date +%s)"
RTO_SECONDS="$((END_EPOCH - START_EPOCH))"
BACKUP_STAMP="$(basename "$BACKUP_DIR")"
if BACKUP_EPOCH="$(date -u -d "${BACKUP_STAMP:0:8} ${BACKUP_STAMP:9:2}:${BACKUP_STAMP:11:2}:${BACKUP_STAMP:13:2}" +%s 2>/dev/null)"; then
  RPO_SECONDS="$((START_EPOCH - BACKUP_EPOCH))"
else
  RPO_SECONDS=-1
fi

DRILL_SUCCEEDED=true
log "service-level restore passed"
log "backup=${BACKUP_STAMP} backup_commit=${BACKUP_COMMIT:0:12} counts(users/posts/media/publish_jobs)=${COUNTS}"
log "RTO=${RTO_SECONDS}s RPO=${RPO_SECONDS}s media_files=${MEDIA_COUNT} publish_exercised=$([[ "$SKIP_PUBLISH" == "true" ]] && printf no || printf yes)"
