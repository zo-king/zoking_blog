#!/usr/bin/env bash
set -Eeuo pipefail

BACKUP_DIR="${1:-}"

if [[ -z "$BACKUP_DIR" ]]; then
  printf 'usage: %s /var/backups/zoking-blog/daily/<timestamp>\n' "$0" >&2
  exit 2
fi

BACKUP_DIR="$(readlink -f "$BACKUP_DIR")"
[[ -d "$BACKUP_DIR" ]] || { printf 'backup directory not found: %s\n' "$BACKUP_DIR" >&2; exit 1; }
[[ -f "$BACKUP_DIR/SHA256SUMS" ]] || { printf 'SHA256SUMS is missing\n' >&2; exit 1; }

for required in postgres.dump media-data.tar.gz site-releases.tar.gz publisher-site.tar.gz goatcounter-data.tar.gz config/env.prod git-commit.txt; do
  [[ -s "$BACKUP_DIR/$required" ]] || { printf 'required backup artifact is missing or empty: %s\n' "$required" >&2; exit 1; }
done

(
  cd "$BACKUP_DIR"
  sha256sum -c SHA256SUMS
)

tar -tzf "$BACKUP_DIR/media-data.tar.gz" >/dev/null
tar -tzf "$BACKUP_DIR/site-releases.tar.gz" >/dev/null
tar -tzf "$BACKUP_DIR/publisher-site.tar.gz" >/dev/null
tar -tzf "$BACKUP_DIR/goatcounter-data.tar.gz" >/dev/null

printf '[zoking-backup] verified %s\n' "$BACKUP_DIR"
