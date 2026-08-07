#!/usr/bin/env bash
set -Eeuo pipefail

BACKUP_DIR="${1:-}"
AGE_IDENTITY="${BACKUP_AGE_IDENTITY:-}"
TEMP_DIR=""

if [[ -z "$BACKUP_DIR" ]]; then
  printf 'usage: %s /var/backups/zoking-blog/daily/<timestamp>\n' "$0" >&2
  exit 2
fi

BACKUP_DIR="$(readlink -f "$BACKUP_DIR")"
[[ -d "$BACKUP_DIR" ]] || { printf 'backup directory not found: %s\n' "$BACKUP_DIR" >&2; exit 1; }
[[ -f "$BACKUP_DIR/SHA256SUMS" ]] || { printf 'SHA256SUMS is missing\n' >&2; exit 1; }
command -v age >/dev/null 2>&1 || { printf 'age is required\n' >&2; exit 1; }

if find "$BACKUP_DIR" -type f ! -name SHA256SUMS ! -name '*.age' -print -quit | grep -q .; then
  printf 'plaintext backup artifact detected\n' >&2
  exit 1
fi

for required in postgres.dump.age media-data.tar.gz.age site-releases.tar.gz.age publisher-site.tar.gz.age goatcounter-data.tar.gz.age config/env.prod.age git-commit.txt.age CONTENT-SHA256SUMS.age; do
  [[ -s "$BACKUP_DIR/$required" ]] || { printf 'required encrypted backup artifact is missing or empty: %s\n' "$required" >&2; exit 1; }
done

(
  cd "$BACKUP_DIR"
  sha256sum -c SHA256SUMS
)

if [[ -n "$AGE_IDENTITY" ]]; then
  [[ -r "$AGE_IDENTITY" ]] || { printf 'age identity is not readable: %s\n' "$AGE_IDENTITY" >&2; exit 1; }
  TEMP_DIR="$(mktemp -d)"
  trap 'rm -rf -- "$TEMP_DIR"' EXIT

  age --decrypt --identity "$AGE_IDENTITY" --output "$TEMP_DIR/CONTENT-SHA256SUMS" \
    "$BACKUP_DIR/CONTENT-SHA256SUMS.age"
  while read -r checksum path; do
    [[ "$path" == ./* ]] || { printf 'invalid content manifest path: %s\n' "$path" >&2; exit 1; }
    output="$TEMP_DIR/$path"
    mkdir -p "$(dirname "$output")"
    age --decrypt --identity "$AGE_IDENTITY" --output "$output" "$BACKUP_DIR/$path.age"
  done < <(sed 's/[[:space:]]\+/ /' "$TEMP_DIR/CONTENT-SHA256SUMS" | awk '{print $1, $2}')

  (
    cd "$TEMP_DIR"
    sha256sum -c CONTENT-SHA256SUMS
  )

  tar -tzf "$TEMP_DIR/./media-data.tar.gz" >/dev/null
  tar -tzf "$TEMP_DIR/./site-releases.tar.gz" >/dev/null
  tar -tzf "$TEMP_DIR/./publisher-site.tar.gz" >/dev/null
  tar -tzf "$TEMP_DIR/./goatcounter-data.tar.gz" >/dev/null
else
  printf '[zoking-backup] encrypted manifest verified; provide BACKUP_AGE_IDENTITY for content verification\n'
fi

printf '[zoking-backup] verified %s\n' "$BACKUP_DIR"
