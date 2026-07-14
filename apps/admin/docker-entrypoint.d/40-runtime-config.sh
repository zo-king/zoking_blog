#!/bin/sh
set -eu

api_base_url="${ADMIN_API_BASE_URL:-http://localhost:18080}"
site_base_url="${ADMIN_SITE_BASE_URL:-https://zoking.tech/}"

validate_http_url() {
  name="$1"
  value="$2"
  if ! printf '%s' "$value" | grep -Eq '^https?://(\[[0-9A-Fa-f:.]+\]|[A-Za-z0-9.-]+)(:[0-9]{1,5})?(/[A-Za-z0-9._~!$&()*+,=:@%/-]*)?$'; then
    echo "$name must be an http(s) URL without a query string or fragment" >&2
    exit 1
  fi
}

validate_http_url ADMIN_API_BASE_URL "$api_base_url"
validate_http_url ADMIN_SITE_BASE_URL "$site_base_url"

api_base_url="${api_base_url%/}"
api_origin="$(printf '%s' "$api_base_url" | sed -E 's#^(https?://(\[[0-9A-Fa-f:.]+\]|[A-Za-z0-9.-]+)(:[0-9]{1,5})?).*$#\1#')"

mkdir -p /etc/nginx/snippets
cat > /etc/nginx/snippets/admin-security-headers.conf <<EOF
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "DENY" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header X-Robots-Tag "noindex, nofollow, noarchive, nosnippet" always;
add_header Content-Security-Policy "default-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' $api_origin" always;
EOF

cat > /usr/share/nginx/html/runtime-config.js <<EOF
window.__ZOKING_ADMIN_CONFIG__ = {
  apiBaseUrl: "$api_base_url",
  siteBaseUrl: "$site_base_url"
};
EOF
