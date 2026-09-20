#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
fail=0

need() {
  if [[ ! -f "$root/$1" ]]; then
    echo "missing: $1" >&2
    fail=1
  fi
}

need cloudflare/wrangler.jsonc
need cloudflare/src/index.ts
need cloudflare/package.json
need cloudflare/LICENSE
need scripts/cf-deploy.sh
need examples/client.toml
need examples/lan-exit.toml
need docs/tunnel.md

python3 - <<PY
import json, pathlib, re, sys
root = pathlib.Path("$root")

wrangler = (root / "cloudflare/wrangler.jsonc").read_text()
# strip // comments for a rough parse
stripped = re.sub(r"^\s*//.*$", "", wrangler, flags=re.M)
data = json.loads(stripped)
if data.get("name") != "easytier-center":
    print("wrangler name must be easytier-center", file=sys.stderr)
    sys.exit(1)
if data.get("main") != "src/index.ts":
    print("wrangler main must be src/index.ts", file=sys.stderr)
    sys.exit(1)
bindings = data.get("durable_objects", {}).get("bindings", [])
if not any(b.get("class_name") == "EasyTierServer" for b in bindings):
    print("missing EasyTierServer durable object", file=sys.stderr)
    sys.exit(1)

readme = (root / "README.md").read_text()
for needle in ("wrangler", "secure-mode", "cf-deploy.sh", "easytier-gui", "初始节点", ":443", "workers.dev"):
    if needle not in readme:
        print(f"README missing {needle!r}", file=sys.stderr)
        sys.exit(1)

client = (root / "examples/client.toml").read_text()
if "[secure_mode]" not in client or "enabled = true" not in client:
    print("client.toml must use [secure_mode] enabled = true", file=sys.stderr)
    sys.exit(1)
if re.search(r"(?m)^secure_mode = true\b", client):
    print("client.toml must not use boolean secure_mode = true", file=sys.stderr)
    sys.exit(1)
if "wss://" not in client or ":443" not in client:
    print("client.toml must use wss :443 peer", file=sys.stderr)
    sys.exit(1)
if "local_private_key" not in client:
    print("client.toml must set secure-mode local keys", file=sys.stderr)
    sys.exit(1)

print("cloudflare worker config ok")
PY

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
