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

cd "$repo_root"
docker compose exec -T postgres psql -U "$db_user" -d "$db_name" "$@"
