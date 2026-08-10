#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

REPO_DIR="${REPO_DIR:-/opt/zoking-blog}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/zoking-blog}"
ACCOUNT="zoking"
NEW_PASSWORD=""
CONFIRM_PASSWORD=""
PASSWORD_B64=""
ACCOUNT_B64=""
LATEST_BACKUP=""

usage() {
  cat <<'EOF'
Usage:
  sudo scripts/ops/reset-admin-password.sh [--account USERNAME_OR_EMAIL]

The command creates and verifies an encrypted backup before and after the
password reset. It updates exactly one active super administrator, revokes all
of that user's refresh tokens, and records a password-reset audit event.
EOF
}

log() {
  printf '[zoking-password-reset] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

cleanup() {
  NEW_PASSWORD=""
  CONFIRM_PASSWORD=""
  PASSWORD_B64=""
  ACCOUNT_B64=""
}
trap cleanup EXIT INT TERM

while (($# > 0)); do
  case "$1" in
    --account)
      (($# >= 2)) || fail "--account requires a username or email"
      ACCOUNT="$2"
      shift 2
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

[[ "$(id -u)" -eq 0 ]] || fail "run as root so protected backups can be created and verified"
[[ -r /dev/tty && -w /dev/tty ]] || fail "an interactive terminal is required"
for command_name in cat docker openssl readlink systemctl tr wc; do
  command -v "$command_name" >/dev/null 2>&1 || fail "missing command: $command_name"
done

REPO_DIR="$(readlink -f "$REPO_DIR")"
BACKUP_ROOT="$(readlink -m "$BACKUP_ROOT")"
ENV_FILE="${REPO_DIR}/infra/docker/.env.prod"
COMPOSE_FILE="${REPO_DIR}/infra/docker/compose.prod.yml"
VERIFY_SCRIPT="${REPO_DIR}/scripts/ops/verify-backup.sh"
RESET_SQL="${REPO_DIR}/scripts/ops/reset-admin-password.sql"

[[ -d "$REPO_DIR/.git" ]] || fail "repository not found: $REPO_DIR"
[[ -r "$ENV_FILE" ]] || fail "production env file is unavailable"
[[ -r "$COMPOSE_FILE" ]] || fail "production Compose file is unavailable"
[[ -x "$VERIFY_SCRIPT" ]] || fail "backup verifier is unavailable"
[[ -r "$RESET_SQL" ]] || fail "password reset SQL is unavailable"
case "$BACKUP_ROOT" in
  /var/backups/zoking-blog|/var/backups/zoking-blog/*) ;;
  *) fail "BACKUP_ROOT must resolve below /var/backups/zoking-blog"
esac

ACCOUNT="$(printf '%s' "$ACCOUNT" | tr '[:upper:]' '[:lower:]')"
[[ -n "$ACCOUNT" && "$ACCOUNT" != *$'\n'* && "$ACCOUNT" != *$'\r'* ]] || fail "account is invalid"
(( ${#ACCOUNT} <= 254 )) || fail "account is too long"

COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
PRODUCTION_API_ID="$("${COMPOSE[@]}" ps -q api)"
PRODUCTION_POSTGRES_ID="$("${COMPOSE[@]}" ps -q postgres)"
[[ -n "$PRODUCTION_API_ID" && -n "$PRODUCTION_POSTGRES_ID" ]] || fail "production API and PostgreSQL must be running"
[[ "$(docker inspect -f '{{.State.Running}}' "$PRODUCTION_API_ID")" == "true" ]] || fail "production API is not running"
[[ "$(docker inspect -f '{{.State.Health.Status}}' "$PRODUCTION_POSTGRES_ID")" == "healthy" ]] || fail "production PostgreSQL is not healthy"

run_backup() {
  local phase="$1"
  local before=""
  local after=""

  before="$(readlink -f "${BACKUP_ROOT}/daily/latest" 2>/dev/null || true)"
  log "starting ${phase} encrypted backup"
  systemctl start zoking-backup.service || fail "${phase} backup service failed"
  [[ "$(systemctl show zoking-backup.service -p Result --value)" == "success" ]] || fail "${phase} backup result is not success"
  after="$(readlink -f "${BACKUP_ROOT}/daily/latest" 2>/dev/null || true)"
  [[ -n "$after" && -d "$after" ]] || fail "${phase} backup latest directory is unavailable"
  [[ "$after" != "$before" ]] || fail "${phase} backup did not create a new snapshot"
  "$VERIFY_SCRIPT" "$after"
  LATEST_BACKUP="$after"
  log "${phase} backup verified: $(basename "$after")"
}

run_backup pre-reset

printf 'New blog admin password for %s (16-72 UTF-8 bytes): ' "$ACCOUNT" >/dev/tty
IFS= read -r -s NEW_PASSWORD </dev/tty
printf '\n' >/dev/tty
printf 'Confirm new blog admin password: ' >/dev/tty
IFS= read -r -s CONFIRM_PASSWORD </dev/tty
printf '\n' >/dev/tty

[[ "$NEW_PASSWORD" == "$CONFIRM_PASSWORD" ]] || fail "password confirmation does not match"
PASSWORD_BYTES="$(LC_ALL=C printf '%s' "$NEW_PASSWORD" | wc -c | tr -d '[:space:]')"
[[ "$PASSWORD_BYTES" =~ ^[0-9]+$ ]] || fail "could not measure password length"
((PASSWORD_BYTES >= 16 && PASSWORD_BYTES <= 72)) || fail "password must contain 16 to 72 UTF-8 bytes"
case "${NEW_PASSWORD,,}" in
  *change-me*|*changeme*|*password123*) fail "password is an obvious placeholder" ;;
esac

ACCOUNT_B64="$(printf '%s' "$ACCOUNT" | openssl base64 -A)"
PASSWORD_B64="$(printf '%s' "$NEW_PASSWORD" | openssl base64 -A)"
NEW_PASSWORD=""
CONFIRM_PASSWORD=""

log "resetting password and revoking refresh tokens"
{
  cat <<'SQL'
begin;
create temp table zoking_password_reset_input (
  account_b64 text not null,
  password_b64 text not null
) on commit drop;
copy zoking_password_reset_input (account_b64, password_b64) from stdin;
SQL
  printf '%s\t%s\n' "$ACCOUNT_B64" "$PASSWORD_B64"
  printf '\\.\n'
  cat "$RESET_SQL"
} | "${COMPOSE[@]}" exec -T postgres sh -ec 'exec psql -X --set ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

PASSWORD_B64=""
ACCOUNT_B64=""

run_backup post-reset

[[ "$(docker inspect -f '{{.State.Running}}' "$PRODUCTION_API_ID")" == "true" ]] || fail "production API stopped during password reset"
[[ "$(docker inspect -f '{{.State.Health.Status}}' "$PRODUCTION_POSTGRES_ID")" == "healthy" ]] || fail "production PostgreSQL is not healthy after password reset"

log "password reset completed; all previous admin refresh tokens were revoked"
log "post-reset recovery source: ${LATEST_BACKUP}"
