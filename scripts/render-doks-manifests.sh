#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
config_file=${1:-"$repo_root/.deployment_config/doks.env"}
template_file="$repo_root/deploy/doks/as-stack.yaml.tmpl"
output_file="$repo_root/.deployment_config/as-stack.rendered.yaml"

if [[ ! -f "$config_file" ]]; then
  printf 'Missing deployment config: %s\n' "$config_file" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
. "$config_file"
set +a

mkdir -p "$(dirname "$output_file")"
envsubst <"$template_file" >"$output_file"
printf '%s\n' "$output_file"
