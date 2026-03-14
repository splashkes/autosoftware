#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
docker_app="/Applications/Docker.app"
max_wait_seconds=${AS_POSTGRES_DOCKER_WAIT_SECONDS:-180}

if [ -x /opt/homebrew/bin/docker ]; then
  docker_bin=/opt/homebrew/bin/docker
elif [ -x /usr/local/bin/docker ]; then
  docker_bin=/usr/local/bin/docker
else
  docker_bin=docker
fi

if [ -f "$repo_root/.env.postgres" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$repo_root/.env.postgres"
  set +a
fi

if ! "$docker_bin" info >/dev/null 2>&1; then
  if [ -d "$docker_app" ]; then
    open -gj -a "$docker_app"
  fi
fi

elapsed=0
until "$docker_bin" info >/dev/null 2>&1; do
  if [ "$elapsed" -ge "$max_wait_seconds" ]; then
    echo "Timed out waiting for Docker Desktop after ${max_wait_seconds}s" >&2
    exit 1
  fi

  sleep 2
  elapsed=$((elapsed + 2))
done

cd "$repo_root"
exec "$docker_bin" compose up -d postgres
