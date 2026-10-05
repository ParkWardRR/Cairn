#!/usr/bin/env bash
set -euo pipefail

HOST="alfa@cairn.alpina.casa"
UI_DIR="/srv/cairn-ui"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Building Nuxt UI..."
cd "$SCRIPT_DIR/../ui"
npm run build

echo "==> Uploading build to $HOST:$UI_DIR..."
ssh "$HOST" "sudo mkdir -p $UI_DIR && sudo chown cairn:cairn $UI_DIR"
rsync -az --delete --rsync-path="sudo -u cairn rsync" .output/ "$HOST:$UI_DIR/.output/"
rsync -az --rsync-path="sudo -u cairn rsync" package.json "$HOST:$UI_DIR/package.json"

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

echo "==> Done. UI available at https://cairn.alpina.casa"
