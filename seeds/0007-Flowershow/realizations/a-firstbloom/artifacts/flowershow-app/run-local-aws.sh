#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CREDS_DIR="${FLOWERSHOW_CREDS_DIR:-$HOME/creds}"

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "missing required file: $path" >&2
    exit 1
  fi
}

require_file "$CREDS_DIR/flower_aws_access"
require_file "$CREDS_DIR/flower_aws_secret"

export AWS_ACCESS_KEY_ID="$(tr -d '\r\n' <"$CREDS_DIR/flower_aws_access")"
export AWS_SECRET_ACCESS_KEY="$(tr -d '\r\n' <"$CREDS_DIR/flower_aws_secret")"
export AWS_REGION="us-east-2"
export AS_S3_BUCKET="flowershow-media-741375879542"
export AS_COGNITO_USER_POOL_ID="us-east-2_rsf8nxr0G"
export AS_COGNITO_CLIENT_ID="6jb5gppphg086vel9us7ehneoa"
export AS_COGNITO_DOMAIN="https://flowershow-741375879542.auth.us-east-2.amazoncognito.com"
export AS_COGNITO_REDIRECT_URL="https://autosoftware.app/flowershow/auth/callback"
export AS_COGNITO_LOGOUT_URL="https://autosoftware.app/flowershow/"

cd "$ROOT"
exec go run . "$@"
