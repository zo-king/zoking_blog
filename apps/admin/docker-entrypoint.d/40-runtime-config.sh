#!/bin/sh
# Generates the runtime config the admin SPA reads at boot
# (window.__ZOKING_ADMIN_CONFIG__.apiBaseUrl, consumed by src/api/client.ts).
# Runs automatically via nginx's /docker-entrypoint.d/ mechanism at container start.
set -eu

API_BASE="${ADMIN_API_BASE_URL:-}"
TARGET="/usr/share/nginx/html/runtime-config.js"

case "$API_BASE" in
  "") ;;
  http://*|https://*) ;;
  *) echo "[40-runtime-config] ADMIN_API_BASE_URL must be an http(s) URL" >&2; exit 1 ;;
esac
case "$API_BASE" in *'"'*|*'\\'*) echo "[40-runtime-config] ADMIN_API_BASE_URL contains unsafe characters" >&2; exit 1 ;; esac
ESCAPED_API_BASE=$(printf '%s' "$API_BASE" | sed 's/[\\\"]/\\\\&/g')

cat > "$TARGET" <<EOF
window.__ZOKING_ADMIN_CONFIG__ = {
  apiBaseUrl: "${ESCAPED_API_BASE}",
};
EOF

echo "[40-runtime-config] wrote ${TARGET} (apiBaseUrl=${API_BASE:-<empty>})"
