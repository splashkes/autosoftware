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

log_dir=${AS_CI_LOG_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/as-ci-kernel-smoke.XXXXXX")}
declare -a service_pids=()

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

dump_logs() {
  local log_file

  for log_file in "$log_dir"/*.log; do
    [[ -f "$log_file" ]] || continue
    printf '\n==> %s\n' "$(basename "$log_file")"
    tail -n 200 "$log_file" || true
  done
}

cleanup() {
  local exit_code=${1:-0}
  local pid

  trap - EXIT INT TERM

  for pid in "${service_pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done

  wait 2>/dev/null || true
  "$repo_root/scripts/as-postgres-down.sh" >/dev/null 2>&1 || true

  if [[ "$exit_code" -ne 0 ]]; then
    printf '\nKernel stack smoke failed. Logs from %s follow.\n' "$log_dir" >&2
    dump_logs >&2
  else
    printf 'Kernel stack smoke passed. Logs written to %s\n' "$log_dir"
  fi

  exit "$exit_code"
}

trap 'cleanup $?' EXIT
trap 'cleanup 1' INT TERM

wait_for_http() {
  local name=$1
  local url=$2
  local attempts=${3:-60}
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

assert_http_contains() {
  local label=$1
  local url=$2
  local pattern=$3
  local body

  body=$(curl -fsS "$url")
  if ! printf '%s' "$body" | grep -Eq "$pattern"; then
    printf 'Expected %s (%s) to contain %s\n' "$label" "$url" "$pattern" >&2
    printf '%s\n' "$body" >&2
    return 1
  fi

  printf '==> %s matched %s\n' "$label" "$pattern"
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

  wait_for_http "$name" "http://$address$health_path"
}

require_command curl
require_command docker
require_command go
require_command python3

mkdir -p "$log_dir"

printf '==> Kernel stack smoke log directory: %s\n' "$log_dir"
printf '==> Runtime database URL: %s\n' "$runtime_database_url"

"$repo_root/scripts/as-postgres-up.sh"

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
  AS_WEBD_ADDR="$webd_addr"

assert_http_contains "runtime health" "http://$apid_addr/v1/runtime/health" '"status":"ok"'
assert_http_contains "contracts" "http://$apid_addr/v1/contracts" '"reference":"0003-customer-service-app/a-web-mvp"'
assert_http_contains "registry status" "http://$registryd_addr/v1/registry/status" 'status'
assert_http_contains "materializer realizations" "http://$materializerd_addr/v1/realizations" 'sources'
assert_http_contains "webd boot surface" "http://$webd_addr/" 'Software that evolves from within\.'
assert_http_contains "growth seed packet" "http://$apid_addr/v1/projections/realization-growth/seed-packet?reference=0003-customer-service-app/a-web-mvp" 'customer-service-app'

printf '==> Queueing growth job\n'
growth_response=$(curl -fsS -X POST "http://$apid_addr/v1/commands/realizations.grow" \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  --data "{\"reference\":\"0003-customer-service-app/a-web-mvp\",\"operation\":\"grow\",\"profile\":\"minimal\",\"target\":\"runnable_mvp\",\"developer_instructions\":\"Smoke test the normalized growth contract from GitHub Actions.\",\"idempotency_key\":\"ci-kernel-smoke-$(date +%s)\"}")

growth_job_id=$(printf '%s' "$growth_response" | python3 -c 'import json, sys; print(json.load(sys.stdin)["job"]["job_id"])')
if [[ -z "$growth_job_id" ]]; then
  printf 'Growth job ID was empty\n' >&2
  exit 1
fi

assert_http_contains "growth job projection" "http://$apid_addr/v1/projections/realization-growth/jobs/$growth_job_id" "$growth_job_id"
assert_http_contains "template materialization" "http://$materializerd_addr/v1/materializations?reference=9999-template/a-author-approach" '9999-template'
