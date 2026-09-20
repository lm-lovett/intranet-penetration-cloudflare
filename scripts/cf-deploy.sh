#!/usr/bin/env bash
# Deploy the EasyTier center node to Cloudflare Workers. No Docker, no VPS.
#
# First time:
#   ./scripts/cf-deploy.sh login
#   ./scripts/cf-deploy.sh secrets
#   ./scripts/cf-deploy.sh
#
# Non-interactive (CI or already-logged wrangler):
#   EASYTIER_NETWORK_NAME=office \
#   EASYTIER_NETWORK_SECRET='long-random-secret' \
#   CLOUDFLARE_API_TOKEN=... \
#   ./scripts/cf-deploy.sh secrets
#   ./scripts/cf-deploy.sh

set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cf="$root/cloudflare"
cd "$cf"

if ! command -v pnpm >/dev/null 2>&1; then
  echo "install pnpm first: npm install -g pnpm" >&2
  exit 1
fi

pnpm install

cmd="${1:-deploy}"

case "$cmd" in
  login)
    pnpm exec wrangler login
    ;;
  keys)
    pnpm run keys
    ;;
  secrets)
    if [[ ! -f "$cf/.keys" ]]; then
      echo "generating X25519 server identity into cloudflare/.keys"
      pnpm run keys | tee "$cf/.keys"
    fi
    # shellcheck disable=SC1091
    set -a
    # .keys is KEY=value from generate-keypair.mjs
    source "$cf/.keys"
    set +a
    network_name="${EASYTIER_NETWORK_NAME:-office}"
    network_secret="${EASYTIER_NETWORK_SECRET:-}"
    if [[ -z "$network_secret" ]]; then
      echo "set EASYTIER_NETWORK_SECRET to a long random string" >&2
      exit 1
    fi
    networks_json="$(
      n="$network_name" s="$network_secret" python3 -c 'import json,os; print(json.dumps([{"network_name":os.environ["n"],"network_secret":os.environ["s"]}]))'
    )"
    printf '%s' "$networks_json" | pnpm exec wrangler secret put EASYTIER_NETWORKS
    printf '%s' "$LOCAL_PRIVATE_KEY" | pnpm exec wrangler secret put LOCAL_PRIVATE_KEY
    printf '%s' "$LOCAL_PUBLIC_KEY" | pnpm exec wrangler secret put LOCAL_PUBLIC_KEY
    echo "secrets uploaded. public key (pin this on clients):"
    echo "  $LOCAL_PUBLIC_KEY"
    ;;
  whoami)
    pnpm exec wrangler whoami
    ;;
  deploy|"")
    pnpm run deploy
    ;;
  dry-run)
    pnpm run build
    ;;
  *)
    echo "usage: $0 [login|keys|secrets|whoami|deploy|dry-run]" >&2
    exit 1
    ;;
esac
