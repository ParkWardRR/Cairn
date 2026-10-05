#!/usr/bin/env bash
# Builds the web UI and installs it on the server.
#
#   deploy/deploy-ui.sh            build the committed code (HEAD) and deploy it
#   deploy/deploy-ui.sh --dirty    build the working tree, uncommitted changes and all
#
# The default builds from a clean export of HEAD so that what is deployed is what
# is committed. Building the working tree ships anything half-finished in it, and
# the UI talks to a server whose schema may not have caught up with that work.
set -euo pipefail

HOST="alfa@cairn.alpina.casa"
UI_DIR="/srv/cairn-ui"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ "${1:-}" = "--dirty" ]; then
  echo "==> Building Nuxt UI from the working tree (uncommitted changes included)..."
  BUILD_DIR="$ROOT/ui"
  (cd "$BUILD_DIR" && npm run build)
else
  echo "==> Building Nuxt UI from $(git -C "$ROOT" rev-parse --short HEAD)..."
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  git -C "$ROOT" archive HEAD ui | tar -x -C "$TMP"
  BUILD_DIR="$TMP/ui"
  # Reuse the installed dependencies. A copy (cloned where the filesystem can),
  # not a symlink, which the build does not resolve correctly.
  cp -Rc "$ROOT/ui/node_modules" "$BUILD_DIR/node_modules" 2>/dev/null || cp -R "$ROOT/ui/node_modules" "$BUILD_DIR/node_modules"
  (cd "$BUILD_DIR" && npx nuxt prepare >/dev/null && npm run build)
  if [ -n "$(git -C "$ROOT" status --porcelain -- ui)" ]; then
    echo "    note: ui/ has uncommitted changes; they are NOT in this deploy"
  fi
fi

echo "==> Uploading build to $HOST:$UI_DIR..."
ssh "$HOST" "sudo mkdir -p $UI_DIR && sudo chown cairn:cairn $UI_DIR"
rsync -az --delete --rsync-path="sudo -u cairn rsync" "$BUILD_DIR/.output/" "$HOST:$UI_DIR/.output/"
rsync -az --rsync-path="sudo -u cairn rsync" "$BUILD_DIR/package.json" "$HOST:$UI_DIR/package.json"

echo "==> Installing systemd service..."
# The unit is only installed when absent: the live one carries host-specific
# settings (map keys) that are not in the repo, so a deploy must not replace it.
scp "$SCRIPT_DIR/systemd/cairn-ui.service" "$HOST:/tmp/cairn-ui.service"
ssh "$HOST" "test -f /etc/systemd/system/cairn-ui.service && rm /tmp/cairn-ui.service || sudo mv /tmp/cairn-ui.service /etc/systemd/system/cairn-ui.service"
scp "$SCRIPT_DIR/systemd/cairn-ui-places.conf" "$HOST:/tmp/places.conf"
ssh "$HOST" "sudo mkdir -p /etc/systemd/system/cairn-ui.service.d && sudo mv /tmp/places.conf /etc/systemd/system/cairn-ui.service.d/places.conf && sudo systemctl daemon-reload"

echo "==> Restarting cairn-ui..."
ssh "$HOST" "sudo systemctl enable --now cairn-ui && sudo systemctl restart cairn-ui"

echo "==> Updating Caddy config..."
scp "$SCRIPT_DIR/caddy/Caddyfile" "$HOST:/tmp/Caddyfile"
ssh "$HOST" "sudo mv /tmp/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy"

echo "==> Checking the API..."
sleep 4
B="$(ssh "$HOST" "curl -s localhost:3000/api/trips" | sed -n 's/.*"boot_id":"\([0-9a-f]*\)".*/\1/p' | head -1)"
fail=0
for p in trips places dashboard/stats heatmap ${B:+trips/$B trips/$B/stops trips/$B/fuel}; do
  code="$(ssh "$HOST" "curl -s -o /dev/null -w '%{http_code}' localhost:3000/api/$p")"
  [ "$code" = "200" ] || { echo "    FAILED /api/$p -> $code"; fail=1; }
done
[ "$fail" = 0 ] || { echo "==> The deploy is up but some endpoints are failing."; exit 1; }

echo "==> Done. UI available at https://cairn.alpina.casa"
