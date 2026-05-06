#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${LIVE_REDTEAM_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
WAVE="${LIVE_REDTEAM_WAVE:-all}"
SCENARIO_ID="${LIVE_REDTEAM_SCENARIO:-}"
HOST_FILTER="${LIVE_REDTEAM_HOST:-}"
OUTPUT_DIR="${LIVE_REDTEAM_OUTPUT_DIR:-$ROOT_DIR/plans/live-anonymous-red-team/reports/runs/$RUN_ID}"
HTTP_ONLY=0
BROWSER_ONLY=0
INCLUDE_MANUAL="${LIVE_REDTEAM_INCLUDE_MANUAL:-0}"
ALLOW_OFF_WINDOW="${LIVE_REDTEAM_ALLOW_OFF_WINDOW:-0}"
BASELINE_PATH="${LIVE_REDTEAM_BASELINE:-}"
NOTE="${LIVE_REDTEAM_NOTE:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --wave)
      WAVE="${2:-all}"
      shift 2
      ;;
    --scenario)
      SCENARIO_ID="${2:-}"
      shift 2
      ;;
    --host)
      HOST_FILTER="${2:-}"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --baseline)
      BASELINE_PATH="${2:-}"
      shift 2
      ;;
    --include-manual)
      INCLUDE_MANUAL=1
      shift
      ;;
    --allow-off-window)
      ALLOW_OFF_WINDOW=1
      shift
      ;;
    --http-only)
      HTTP_ONLY=1
      shift
      ;;
    --browser-only)
      BROWSER_ONLY=1
      shift
      ;;
    --note)
      NOTE="${2:-}"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ "$HTTP_ONLY" -eq 1 && "$BROWSER_ONLY" -eq 1 ]]; then
  echo "choose at most one of --http-only or --browser-only" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

COMMON_ENV=(
  "LIVE_REDTEAM_RUN_ID=$RUN_ID"
  "LIVE_REDTEAM_WAVE=$WAVE"
  "LIVE_REDTEAM_SCENARIO=$SCENARIO_ID"
  "LIVE_REDTEAM_HOST=$HOST_FILTER"
  "LIVE_REDTEAM_OUTPUT_DIR=$OUTPUT_DIR"
  "LIVE_REDTEAM_INCLUDE_MANUAL=$INCLUDE_MANUAL"
  "LIVE_REDTEAM_ALLOW_OFF_WINDOW=$ALLOW_OFF_WINDOW"
  "LIVE_REDTEAM_NOTE=$NOTE"
)

if [[ -n "$BASELINE_PATH" ]]; then
  COMMON_ENV+=("LIVE_REDTEAM_BASELINE=$BASELINE_PATH")
fi

if [[ "$BROWSER_ONLY" -eq 0 ]]; then
  HTTP_CMD=(node "$ROOT_DIR/scripts/live-redteam-http.mjs" --wave "$WAVE" --output-dir "$OUTPUT_DIR/http")
  if [[ -n "$SCENARIO_ID" ]]; then
    HTTP_CMD+=(--scenario "$SCENARIO_ID")
  fi
  if [[ -n "$HOST_FILTER" ]]; then
    HTTP_CMD+=(--host "$HOST_FILTER")
  fi
  if [[ "$INCLUDE_MANUAL" -eq 1 ]]; then
    HTTP_CMD+=(--include-manual)
  fi
  if [[ "$ALLOW_OFF_WINDOW" -eq 1 ]]; then
    HTTP_CMD+=(--allow-off-window)
  fi
  if [[ -n "$BASELINE_PATH" ]]; then
    HTTP_CMD+=(--baseline "$BASELINE_PATH")
  fi
  if [[ -n "$NOTE" ]]; then
    HTTP_CMD+=(--note "$NOTE")
  fi
  echo "running HTTP harness: ${HTTP_CMD[*]}"
  env "${COMMON_ENV[@]}" "${HTTP_CMD[@]}"
fi

if [[ "$HTTP_ONLY" -eq 0 ]]; then
  echo "running browser harness: tests/playwright.live.config.ts"
  (
    cd "$ROOT_DIR/tests"
    env "${COMMON_ENV[@]}" npx playwright test -c playwright.live.config.ts
  )
fi

echo "live red-team wave complete: $OUTPUT_DIR"
