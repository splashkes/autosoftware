#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$repo_root"
docker compose down --volumes --remove-orphans
"$repo_root/scripts/as-postgres-up.sh"
