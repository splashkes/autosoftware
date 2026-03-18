#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

declare -a module_dirs=(
  "$repo_root/seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app"
  "$repo_root/seeds/0004-event-listings/realizations/a-web-mvp/artifacts/event-listings-app"
  "$repo_root/seeds/0006-registry-browser/realizations/a-authoritative-browser/artifacts/registry-browser"
  "$repo_root/seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app"
)

for module_dir in "${module_dirs[@]}"; do
  printf '==> go test ./... (%s)\n' "$module_dir"
  (
    cd "$module_dir"
    go test ./...
  )
done
