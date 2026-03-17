#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target_goos=${AS_PREBUILD_GOOS:-linux}
target_goarch=${AS_PREBUILD_GOARCH:-amd64}

mkdir -p "$repo_root/materialized/realizations"

(
  cd "$repo_root/kernel"
  go run ./cmd/prebuildrealizations \
    -repo-root "$repo_root" \
    -goos "$target_goos" \
    -goarch "$target_goarch"
)
