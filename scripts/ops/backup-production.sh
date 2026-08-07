#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

REPO_DIR="${REPO_DIR:-/opt/zoking-blog}"
ENV_FILE="${ENV_FILE:-${REPO_DIR}/infra/docker/.env.prod}"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_DIR}/infra/docker/compose.prod.yml}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/zoking-blog}"
BACKUP_REMOTE="${BACKUP_REMOTE:-}"
BACKUP_SSH_KEY="${BACKUP_SSH_KEY:-}"
DAILY_KEEP_DAYS="${DAILY_KEEP_DAYS:-7}"
WEEKLY_KEEP_DAYS="${WEEKLY_KEEP_DAYS:-35}"
MONTHLY_KEEP_DAYS="${MONTHLY_KEEP_DAYS:-100}"

COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DAILY_ROOT="${BACKUP_ROOT}/daily"
STAGING="${BACKUP_ROOT}/.incomplete-${STAMP}-$$"
FINAL="${DAILY_ROOT}/${STAMP}"

log() {
  printf '[zoking-backup] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

cleanup_staging() {
  case "$STAGING" in
    "${BACKUP_ROOT}"/.incomplete-*) rm -rf -- "$STAGING" ;;
    *) log "refusing to remove unexpected staging path: $STAGING" ;;
  esac
}

volume_name() {
  local logical_name="$1"
  local project
  local container_id

  container_id="$("${COMPOSE[@]}" ps -q postgres)"
  [[ -n "$container_id" ]] || fail "postgres container is not running"
  project="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$container_id")"
  docker volume ls -q \
    --filter "label=com.docker.compose.project=${project}" \
    --filter "label=com.docker.compose.volume=${logical_name}" | head -n 1
}

archive_volume() {
  local logical_name="$1"
  local output_name="$2"
  local volume
  local mountpoint

  volume="$(volume_name "$logical_name")"
  [[ -n "$volume" ]] || fail "could not resolve Compose volume: $logical_name"
  mountpoint="$(docker volume inspect -f '{{ .Mountpoint }}' "$volume")"
  [[ -d "$mountpoint" ]] || fail "volume mountpoint is unavailable: $logical_name"
  tar --numeric-owner -C "$mountpoint" -czf "${STAGING}/${output_name}" .
}

require_command docker
require_command flock
require_command sha256sum
require_command tar
if [[ -n "$BACKUP_REMOTE" ]]; then
  require_command rsync
  if [[ -n "$BACKUP_SSH_KEY" ]]; then
    require_command ssh
    [[ -r "$BACKUP_SSH_KEY" ]] || fail "backup SSH key is not readable: $BACKUP_SSH_KEY"
  fi
fi

[[ "$(id -u)" -eq 0 ]] || fail "run as root so Docker volume data and protected config can be read"
[[ -d "$REPO_DIR/.git" ]] || fail "repository not found: $REPO_DIR"
[[ -r "$ENV_FILE" ]] || fail "production env file is not readable: $ENV_FILE"
[[ -r "$COMPOSE_FILE" ]] || fail "Compose file is not readable: $COMPOSE_FILE"

mkdir -p "$BACKUP_ROOT" "$DAILY_ROOT" "${BACKUP_ROOT}/weekly" "${BACKUP_ROOT}/monthly"
exec 9>"${BACKUP_ROOT}/.backup.lock"
flock -n 9 || fail "another backup is already running"

trap cleanup_staging ERR INT TERM
mkdir -p "$STAGING/config"

log "creating ${STAMP}"
"${COMPOSE[@]}" exec -T postgres sh -ec \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' >"${STAGING}/postgres.dump"

archive_volume media_data media-data.tar.gz
archive_volume site_releases site-releases.tar.gz
archive_volume publisher_site publisher-site.tar.gz
archive_volume goatcounter_data goatcounter-data.tar.gz

install -m 0600 "$ENV_FILE" "${STAGING}/config/env.prod"
install -m 0600 "$COMPOSE_FILE" "${STAGING}/config/compose.prod.yml"
git -c safe.directory="$REPO_DIR" -C "$REPO_DIR" rev-parse HEAD >"${STAGING}/git-commit.txt"
git -c safe.directory="$REPO_DIR" -C "$REPO_DIR" status --short --branch >"${STAGING}/git-status.txt"
"${COMPOSE[@]}" ps >"${STAGING}/compose-ps.txt"
"${COMPOSE[@]}" images >"${STAGING}/compose-images.txt"

if [[ -r /etc/wireguard/wg0.conf ]]; then
  install -m 0600 /etc/wireguard/wg0.conf "${STAGING}/config/wg0.conf"
fi

(
  cd "$STAGING"
  find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum >SHA256SUMS
  sha256sum -c SHA256SUMS >/dev/null
)

mv "$STAGING" "$FINAL"
trap - ERR INT TERM
ln -sfn "${STAMP}" "${DAILY_ROOT}/latest"

if [[ "$(date -u +%u)" == "7" ]]; then
  cp -al "$FINAL" "${BACKUP_ROOT}/weekly/${STAMP}"
fi
if [[ "$(date -u +%d)" == "01" ]]; then
  cp -al "$FINAL" "${BACKUP_ROOT}/monthly/${STAMP}"
fi

find "$DAILY_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime "+${DAILY_KEEP_DAYS}" -exec rm -rf -- {} +
find "${BACKUP_ROOT}/weekly" -mindepth 1 -maxdepth 1 -type d -mtime "+${WEEKLY_KEEP_DAYS}" -exec rm -rf -- {} +
find "${BACKUP_ROOT}/monthly" -mindepth 1 -maxdepth 1 -type d -mtime "+${MONTHLY_KEEP_DAYS}" -exec rm -rf -- {} +

if [[ -n "$BACKUP_REMOTE" ]]; then
  log "copying encrypted-in-transit backup to remote target"
  rsync_args=(-a --protect-args)
  if [[ -n "$BACKUP_SSH_KEY" ]]; then
    rsync_args+=(-e "ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -i ${BACKUP_SSH_KEY}")
  fi
  rsync "${rsync_args[@]}" "$FINAL/" "${BACKUP_REMOTE%/}/daily/${STAMP}/"
fi

log "completed ${FINAL}"
