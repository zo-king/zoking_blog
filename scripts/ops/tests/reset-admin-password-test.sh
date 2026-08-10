#!/usr/bin/env bash
set -Eeuo pipefail

DEFAULT_REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
REPO_DIR="${REPO_DIR:-$DEFAULT_REPO_DIR}"
RESET_SQL="${RESET_SQL:-${REPO_DIR}/scripts/ops/reset-admin-password.sql}"
POSTGRES_IMAGE="postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
CONTAINER="zoking-password-reset-test-$$"

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker run -d \
  --name "$CONTAINER" \
  --label zoking.password-reset-test=true \
  --network none \
  --tmpfs /var/lib/postgresql/data \
  --env POSTGRES_DB=zoking_reset_test \
  --env POSTGRES_USER=zoking_test \
  --env POSTGRES_PASSWORD=test-only-password \
  "$POSTGRES_IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pg_isready -U zoking_test -d zoking_reset_test >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$CONTAINER" pg_isready -U zoking_test -d zoking_reset_test >/dev/null

apply_migration_up() {
  sed '/^-- +goose Down/,$d' "$1" |
    docker exec -i "$CONTAINER" psql -X --set ON_ERROR_STOP=1 -U zoking_test -d zoking_reset_test >/dev/null
}

apply_migration_up "${REPO_DIR}/db/migrations/20260710000100_create_core.sql"
apply_migration_up "${REPO_DIR}/db/migrations/20260711000200_create_audit_logs.sql"

docker exec -i "$CONTAINER" psql -X --set ON_ERROR_STOP=1 -U zoking_test -d zoking_reset_test >/dev/null <<'SQL'
insert into roles (code, name, is_system) values ('super_admin', 'Super Admin', true);
insert into users (email, username, password_hash, display_name, status)
values ('owner@example.test', 'zoking', crypt('old-test-password-123', gen_salt('bf', 4)), 'Owner', 'active');
insert into user_roles (user_id, role_id)
select u.id, r.id from users u cross join roles r where u.username = 'zoking' and r.code = 'super_admin';
insert into refresh_tokens (user_id, token_hash, expires_at)
select id, 'test-refresh-token-hash', now() + interval '1 day' from users where username = 'zoking';
SQL

run_reset() {
  local account_b64="$1"
  local password_b64="$2"
  {
    cat <<'SQL'
begin;
create temp table zoking_password_reset_input (
  account_b64 text not null,
  password_b64 text not null
) on commit drop;
copy zoking_password_reset_input (account_b64, password_b64) from stdin;
SQL
    printf '%s\t%s\n' "$account_b64" "$password_b64"
    printf '\\.\n'
    cat "$RESET_SQL"
  } | docker exec -i "$CONTAINER" psql -X --set ON_ERROR_STOP=1 -U zoking_test -d zoking_reset_test
}

new_password='new-test-password-456'
run_reset \
  "$(printf 'zoking' | openssl base64 -A)" \
  "$(printf '%s' "$new_password" | openssl base64 -A)" >/dev/null

result="$(docker exec "$CONTAINER" psql -X -AtF '|' -U zoking_test -d zoking_reset_test -c \
  "select (crypt('${new_password}', password_hash) = password_hash)::text, (select count(*) from refresh_tokens), (select count(*) from audit_logs where action = 'admin.password.reset.ops') from users where username = 'zoking'")"
[[ "$result" == "true|0|1" ]] || { printf 'unexpected reset result: %s\n' "$result" >&2; exit 1; }

if run_reset \
  "$(printf 'missing-admin' | openssl base64 -A)" \
  "$(printf 'another-test-password-789' | openssl base64 -A)" >/dev/null 2>&1; then
  printf 'missing administrator reset unexpectedly succeeded\n' >&2
  exit 1
fi

result="$(docker exec "$CONTAINER" psql -X -AtF '|' -U zoking_test -d zoking_reset_test -c \
  "select (crypt('${new_password}', password_hash) = password_hash)::text, (select count(*) from audit_logs where action = 'admin.password.reset.ops') from users where username = 'zoking'")"
[[ "$result" == "true|1" ]] || { printf 'failed reset was not rolled back: %s\n' "$result" >&2; exit 1; }

printf '[zoking-password-reset-test] passed\n'
