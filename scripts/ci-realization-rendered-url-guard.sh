#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

banned_hosts='(?:localhost|127\.0\.0\.1|0\.0\.0\.0|(?:[A-Za-z0-9-]+\.)?autosoftware\.(?:app|dev))(?::[0-9]+)?'
scan_root='seeds/*/realizations/*/artifacts'
common_args=(
  --glob "$scan_root/**"
  --glob '!**/*_test.go'
  --glob '!**/*.md'
)

failures=0

run_check() {
  local label="$1"
  local pattern="$2"
  local output

  output=$(rg -nH -P "$pattern" "${common_args[@]}" seeds || true)
  if [[ -z "$output" ]]; then
    return
  fi

  failures=1
  printf '\n[%s]\n%s\n' "$label" "$output"
}

run_check \
  "rendered-markup" \
  "(?:href|src|action|formaction|data-copy)=(?:\"|')https?://${banned_hosts}(?:/|[\"'?#])"

run_check \
  "browser-navigation" \
  "(?:window\\.(?:open|location(?:\\.href)?)|location\\.(?:assign|replace))[^\\n]*https?://${banned_hosts}"

run_check \
  "render-view-fields" \
  "\\b[A-Z][A-Za-z0-9_]*URL\\s*:\\s*\"https?://${banned_hosts}(?:/|[\"?#])"

run_check \
  "template-url-casts" \
  "template\\.URL\\(\"https?://${banned_hosts}(?:/|[\"?#])"

if [[ "$failures" -ne 0 ]]; then
  cat <<'EOF'

Rendered realizations must not bake environment-specific absolute app URLs into HTML, JS, or view data.
If a realization needs an absolute URL in the UI, derive it from request/runtime data instead of hardcoding localhost or deployment hosts.
EOF
  exit 1
fi

echo "No baked environment-specific realization URLs found in rendered surfaces."
