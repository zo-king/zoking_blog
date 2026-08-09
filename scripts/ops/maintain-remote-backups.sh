#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/zoking-blog}"
DAILY_KEEP_DAYS="${REMOTE_DAILY_KEEP_DAYS:-7}"
WEEKLY_KEEP_DAYS="${REMOTE_WEEKLY_KEEP_DAYS:-35}"
MONTHLY_KEEP_DAYS="${REMOTE_MONTHLY_KEEP_DAYS:-100}"
DISK_WARN_PERCENT="${REMOTE_DISK_WARN_PERCENT:-80}"
MODE="${1:---check}"

DAILY_ROOT="${BACKUP_ROOT}/daily"
WEEKLY_ROOT="${BACKUP_ROOT}/weekly"
MONTHLY_ROOT="${BACKUP_ROOT}/monthly"

log() {
  printf '[zoking-remote-backup] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

usage() {
  printf 'usage: %s [--check|--prune]\n' "$0" >&2
  exit 2
}

require_number() {
  [[ "$2" =~ ^[0-9]+$ ]] || fail "$1 must be a non-negative integer"
}

valid_stamp() {
  [[ "$1" =~ ^[0-9]{8}T[0-9]{6}Z$ ]]
}

list_stamps() {
  local root="$1" stamp
  {
    while IFS= read -r stamp; do
      if valid_stamp "$stamp"; then
        printf '%s\n' "$stamp"
      fi
    done < <(find "$root" -mindepth 1 -maxdepth 1 -type d -printf '%f\n')
    true
  } | sort
}

latest_stamp() {
  list_stamps "$DAILY_ROOT" | tail -n 1
}

verify_backup() {
  local directory="$1" plaintext

  [[ -f "$directory/SHA256SUMS" ]] || fail "SHA256SUMS is missing: $directory"
  plaintext="$(find "$directory" -type f ! -name SHA256SUMS ! -name '*.age' -print -quit)"
  if [[ -n "$plaintext" ]]; then
    fail "plaintext artifact detected: $directory"
  fi
  (
    cd "$directory"
    sha256sum -c SHA256SUMS >/dev/null
  ) || fail "encrypted manifest verification failed: $directory"
}

ensure_tier_link() {
  local target_root="$1"
  local stamp="$2"
  local target="${target_root}/${stamp}"

  if [[ -e "$target" ]]; then
    [[ -d "$target" ]] || fail "tier target is not a directory: $target"
    return
  fi

  cp -al "${DAILY_ROOT}/${stamp}" "$target"
  log "created retention tier ${target}"
}

materialize_tiers() {
  local stamp date_value weekday day

  while IFS= read -r stamp; do
    date_value="${stamp:0:4}-${stamp:4:2}-${stamp:6:2}"
    weekday="$(date -u -d "$date_value" +%u)"
    day="${stamp:6:2}"
    if [[ "$weekday" == "7" ]]; then
      ensure_tier_link "$WEEKLY_ROOT" "$stamp"
    fi
    if [[ "$day" == "01" ]]; then
      ensure_tier_link "$MONTHLY_ROOT" "$stamp"
    fi
  done < <(list_stamps "$DAILY_ROOT")
}

prune_root() {
  local root="$1"
  local keep_days="$2"
  local protected_stamp="${3:-}"
  local directory stamp date_value epoch now age_seconds

  now="$(date -u +%s)"

  while IFS= read -r directory; do
    stamp="${directory##*/}"
    valid_stamp "$stamp" || continue
    if [[ "$stamp" == "$protected_stamp" ]]; then
      continue
    fi

    date_value="${stamp:0:4}-${stamp:4:2}-${stamp:6:2} ${stamp:9:2}:${stamp:11:2}:${stamp:13:2} UTC"
    if ! epoch="$(date -u -d "$date_value" +%s 2>/dev/null)"; then
      log "skipping backup with invalid timestamp ${directory}"
      continue
    fi
    age_seconds=$((now - epoch))
    if ((age_seconds <= keep_days * 86400)); then
      continue
    fi

    rm -rf -- "$directory"
    log "removed expired backup ${directory}"
  done < <(find "$root" -mindepth 1 -maxdepth 1 -type d -printf '%p\n')
}

check_disk() {
  local used
  used="$(df -P "$BACKUP_ROOT" | awk 'NR == 2 {gsub(/%/, "", $5); print $5}')"
  if [[ "$used" =~ ^[0-9]+$ ]] && ((used < DISK_WARN_PERCENT)); then
    log "disk ${BACKUP_ROOT} ${used}% used"
    return
  fi
  fail "disk ${BACKUP_ROOT} is ${used:-unknown}% used; threshold ${DISK_WARN_PERCENT}%"
}

case "$MODE" in
  --check|--prune) ;;
  *) usage ;;
esac

require_number REMOTE_DAILY_KEEP_DAYS "$DAILY_KEEP_DAYS"
require_number REMOTE_WEEKLY_KEEP_DAYS "$WEEKLY_KEEP_DAYS"
require_number REMOTE_MONTHLY_KEEP_DAYS "$MONTHLY_KEEP_DAYS"
require_number REMOTE_DISK_WARN_PERCENT "$DISK_WARN_PERCENT"
((DISK_WARN_PERCENT > 0 && DISK_WARN_PERCENT <= 100)) || fail "REMOTE_DISK_WARN_PERCENT must be between 1 and 100"

[[ "$(id -u)" -eq 0 ]] || fail "run as root so backup permissions can be verified"
[[ -d "$DAILY_ROOT" ]] || fail "daily backup directory is missing: $DAILY_ROOT"

stamp="$(latest_stamp)"
[[ -n "$stamp" ]] || fail "no timestamped remote daily backup exists"
verify_backup "${DAILY_ROOT}/${stamp}"
log "verified latest encrypted backup ${stamp}"

if [[ "$MODE" == "--prune" ]]; then
  mkdir -p "$WEEKLY_ROOT" "$MONTHLY_ROOT"
  materialize_tiers
  ln -sfn "$stamp" "${DAILY_ROOT}/latest"
  prune_root "$DAILY_ROOT" "$DAILY_KEEP_DAYS" "$stamp"
  prune_root "$WEEKLY_ROOT" "$WEEKLY_KEEP_DAYS"
  prune_root "$MONTHLY_ROOT" "$MONTHLY_KEEP_DAYS"
fi

check_disk
log "completed ${MODE}"
