#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
kernel_root="$repo_root/kernel"

if [[ -f "$repo_root/.env.postgres" ]]; then
  set -a
  # shellcheck disable=SC1091
  . "$repo_root/.env.postgres"
  set +a
fi

db_name=${AS_POSTGRES_DB:-as_local}
db_user=${AS_POSTGRES_USER:-postgres}
db_password=${AS_POSTGRES_PASSWORD:-postgres}
db_port=${AS_POSTGRES_PORT:-54329}
runtime_database_url=${AS_RUNTIME_DATABASE_URL:-postgres://$db_user:$db_password@127.0.0.1:$db_port/$db_name?sslmode=disable}

apid_addr=${AS_APID_ADDR:-127.0.0.1:8092}
registryd_addr=${AS_REGISTRYD_ADDR:-127.0.0.1:8093}
materializerd_addr=${AS_MATERIALIZER_ADDR:-127.0.0.1:8091}
webd_addr=${AS_WEBD_ADDR:-127.0.0.1:8090}
execd_addr=${AS_EXECD_ADDR:-127.0.0.1:8094}

log_dir=${AS_LOCAL_RUN_LOG_DIR:-${AS_LOCAL_DEPLOY_LOG_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/as-local-run.XXXXXX")}}
declare -a service_pids=()
declare -a log_pids=()

mkdir -p "$log_dir"

cleanup() {
  local exit_code=${1:-0}

  trap - EXIT INT TERM

  for pid in "${log_pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done

  for pid in "${service_pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done

  wait 2>/dev/null || true

  printf '\n==> Local run services stopped. Postgres is still running.\n'
  printf '==> Use ./scripts/as-postgres-down.sh if you want to stop the database.\n'
  printf '==> Service logs were written to %s\n' "$log_dir"

  exit "$exit_code"
}

trap 'cleanup 0' INT TERM
trap 'cleanup $?' EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

wait_for_http() {
  local name=$1
  local url=$2
  local attempts=${3:-30}
  local delay_seconds=${4:-1}
  local attempt

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      printf '==> %s is healthy at %s\n' "$name" "$url"
      return 0
    fi
    sleep "$delay_seconds"
  done

  printf 'Timed out waiting for %s at %s\n' "$name" "$url" >&2
  return 1
}

show_http_json() {
  local label=$1
  local url=$2

  printf '\n==> %s\n' "$label"
  curl -fsS "$url"
  printf '\n'
}

show_http_match() {
  local label=$1
  local url=$2
  local pattern=$3

  printf '\n==> %s\n' "$label"
  curl -fsS "$url" | rg -n "$pattern"
  printf '\n'
}

start_log_tail() {
  local name=$1
  local log_file=$2

  (
    tail -n +1 -f "$log_file" 2>/dev/null |
      sed "s/^/[$name] /"
  ) &
  log_pids+=("$!")
}

start_go_service() {
  local name=$1
  local address=$2
  local health_path=$3
  shift 3

  local log_file="$log_dir/$name.log"
  : >"$log_file"

  (
    cd "$kernel_root"
    env "$@" go run "./cmd/$name"
  ) >"$log_file" 2>&1 &
  service_pids+=("$!")

  start_log_tail "$name" "$log_file"
  wait_for_http "$name" "http://$address$health_path"
}

require_command docker
require_command curl
require_command go
require_command python3
require_command rg

printf '==> Local run log directory: %s\n' "$log_dir"
printf '==> Runtime database URL: %s\n' "$runtime_database_url"

printf '\n==> Bootstrapping local Postgres\n'
"$repo_root/scripts/as-postgres-up.sh"

printf '\n==> Recent Postgres container output\n'
(
  cd "$repo_root"
  docker compose logs --tail 20 postgres
)

printf '\n==> Verifying runtime tables exist\n'
PAGER=cat PSQL_PAGER=off "$repo_root/scripts/as-postgres-psql.sh" -c '\dt runtime_*'

printf '\n==> Running kernel tests\n'
(
  cd "$kernel_root"
  go test ./...
)

printf '\n==> Following Postgres container logs\n'
(
  cd "$repo_root"
  docker compose logs -f --tail 5 postgres |
    sed 's/^/[postgres] /'
) &
log_pids+=("$!")

printf '\n==> Starting kernel services\n'
start_go_service apid "$apid_addr" "/v1/runtime/health" \
  AS_RUNTIME_DATABASE_URL="$runtime_database_url" \
  AS_RUNTIME_AUTO_MIGRATE=1 \
  AS_APID_ADDR="$apid_addr"

start_go_service registryd "$registryd_addr" "/healthz" \
  AS_RUNTIME_DATABASE_URL="$runtime_database_url" \
  AS_RUNTIME_AUTO_MIGRATE=1 \
  AS_REGISTRYD_ADDR="$registryd_addr"

start_go_service materializerd "$materializerd_addr" "/healthz" \
  AS_MATERIALIZER_ADDR="$materializerd_addr"

start_go_service webd "$webd_addr" "/" \
  AS_RUNTIME_DATABASE_URL="$runtime_database_url" \
  AS_RUNTIME_AUTO_MIGRATE=1 \
  AS_BOOT_EXECUTION_ENABLED=1 \
  AS_WEBD_ADDR="$webd_addr"

start_go_service execd "$execd_addr" "/healthz" \
  AS_RUNTIME_DATABASE_URL="$runtime_database_url" \
  AS_RUNTIME_AUTO_MIGRATE=1 \
  AS_APID_ADDR="$apid_addr" \
  AS_REGISTRYD_ADDR="$registryd_addr" \
  AS_EXECD_ADDR="$execd_addr"

show_http_json "API runtime health" "http://$apid_addr/v1/runtime/health"
show_http_json "API contracts" "http://$apid_addr/v1/contracts"
show_http_json "Registry status" "http://$registryd_addr/v1/registry/status"
show_http_json "Materializer realizations" "http://$materializerd_addr/v1/realizations"
show_http_match "Web growth console markers" "http://$webd_addr/" "Growth Console|Inspect|Grow|Run|Defined|Runnable"
show_http_json "Growth seed packet" "http://$apid_addr/v1/projections/realization-growth/seed-packet?reference=0003-customer-service-app/a-web-mvp"

printf '\n==> Queue growth job\n'
growth_response=$(curl -fsS -X POST "http://$apid_addr/v1/commands/realizations.grow" \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  --data "{\"reference\":\"0003-customer-service-app/a-web-mvp\",\"operation\":\"grow\",\"profile\":\"minimal\",\"target\":\"runnable_mvp\",\"developer_instructions\":\"Smoke test the normalized growth contract from the local run script.\",\"idempotency_key\":\"local-run-smoke-$(date +%s)\"}")
printf '%s\n' "$growth_response"

growth_job_id=$(printf '%s' "$growth_response" | python3 -c 'import json, sys; print(json.load(sys.stdin)["job"]["job_id"])')
show_http_json "Growth job projection" "http://$apid_addr/v1/projections/realization-growth/jobs/$growth_job_id"
show_http_json "Materialize template realization" "http://$materializerd_addr/v1/materializations?reference=9999-template/a-author-approach"

printf '\n==> Local stack is live\n'
printf '    webd:         http://%s/\n' "$webd_addr"
printf '    materializer: http://%s/v1/realizations\n' "$materializerd_addr"
printf '    apid:         http://%s/v1/contracts\n' "$apid_addr"
printf '    registryd:    http://%s/v1/registry/status\n' "$registryd_addr"
printf '    execd:        http://%s/healthz\n' "$execd_addr"
printf '\n==> Press Ctrl-C to stop the Go services. Postgres will remain running.\n'

while true; do
  for pid in "${service_pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      printf 'A local run service exited unexpectedly. Check logs in %s\n' "$log_dir" >&2
      exit 1
    fi
  done
  sleep 2
done
