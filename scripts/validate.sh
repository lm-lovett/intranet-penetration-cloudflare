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

need docker-compose.yml
need docker-compose.bridge.yml
need docker-compose.quick.yml
need .env.example
need conf/easytier.toml
need conf/easytier.bridge.toml
need examples/client.toml
need examples/lan-exit.toml
need systemd/easytier-center.service
need systemd/cloudflared.service

python3 - <<PY
import pathlib, sys
root = pathlib.Path("$root")

try:
    import yaml
except ImportError:
    yaml = None

required_env = [
    "EASYTIER_NETWORK_NAME",
    "EASYTIER_NETWORK_SECRET",
    "EASYTIER_PUBLIC_HOST",
    "TUNNEL_TOKEN",
]
env_text = (root / ".env.example").read_text()
missing = [key for key in required_env if key not in env_text]
if missing:
    print("env.example missing keys:", ", ".join(missing), file=sys.stderr)
    sys.exit(1)

toml = (root / "conf/easytier.toml").read_text()
for needle in ("ws://127.0.0.1:11011/", "no_tun = true", "private_mode = true", "mapped_listeners"):
    if needle not in toml:
        print(f"easytier.toml missing {needle!r}", file=sys.stderr)
        sys.exit(1)

bridge_toml = (root / "conf/easytier.bridge.toml").read_text()
if "ws://0.0.0.0:11011/" not in bridge_toml:
    print("easytier.bridge.toml must listen on 0.0.0.0", file=sys.stderr)
    sys.exit(1)

if yaml is not None:
    for name in ("docker-compose.yml", "docker-compose.bridge.yml", "docker-compose.quick.yml"):
        with (root / name).open() as fh:
            data = yaml.safe_load(fh)
        if not data or "services" not in data:
            print(f"{name} has no services", file=sys.stderr)
            sys.exit(1)
        if name != "docker-compose.quick.yml" and "easytier" not in data["services"]:
            print(f"{name} missing easytier service", file=sys.stderr)
            sys.exit(1)
        if "cloudflared" not in data["services"]:
            print(f"{name} missing cloudflared service", file=sys.stderr)
            sys.exit(1)
    print("compose yaml ok")
else:
    print("pyyaml not installed; skipped compose parse")

print("validate ok")
PY

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
