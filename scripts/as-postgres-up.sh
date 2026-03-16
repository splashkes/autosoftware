#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ -f "$repo_root/.env.postgres" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$repo_root/.env.postgres"
  set +a
fi

db_name=${AS_POSTGRES_DB:-as_local}
db_user=${AS_POSTGRES_USER:-postgres}
db_port=${AS_POSTGRES_PORT:-54329}

cd "$repo_root"

docker compose up -d postgres

docker compose exec -T postgres sh -lc '
  until pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
    sleep 1
  done
'

for file in "$repo_root"/kernel/db/runtime/*.sql; do
  [ -f "$file" ] || continue

  docker compose exec -T postgres psql \
    -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 \
    -U "$db_user" \
    -d "$db_name" \
    -f "/workspace/kernel/db/runtime/$(basename "$file")"
done

printf 'AS Postgres is ready on localhost:%s (%s)\n' "$db_port" "$db_name"
printf 'DATABASE_URL=postgres://%s:%s@localhost:%s/%s?sslmode=disable\n' \
  "$db_user" \
  "${AS_POSTGRES_PASSWORD:-postgres}" \
  "$db_port" \
  "$db_name"
