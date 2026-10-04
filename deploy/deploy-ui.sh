#!/usr/bin/env bash
set -euo pipefail

HOST="alfa@cairn.alpina.casa"
UI_DIR="/srv/cairn-ui"

echo "==> Building Nuxt UI..."
cd "$(dirname "$0")/../ui"
npm run build

echo "==> Uploading build to $HOST:$UI_DIR..."
ssh "$HOST" "sudo mkdir -p $UI_DIR && sudo chown cairn:cairn $UI_DIR"
rsync -az --delete --rsync-path="sudo -u cairn rsync" .output/ "$HOST:$UI_DIR/.output/"
rsync -az --rsync-path="sudo -u cairn rsync" package.json "$HOST:$UI_DIR/package.json"

echo "==> Installing systemd service..."
scp "$(dirname "$0")/systemd/cairn-ui.service" "$HOST:/tmp/cairn-ui.service"
ssh "$HOST" "sudo mv /tmp/cairn-ui.service /etc/systemd/system/cairn-ui.service && sudo systemctl daemon-reload"

echo "==> Restarting cairn-ui..."
ssh "$HOST" "sudo systemctl enable --now cairn-ui && sudo systemctl restart cairn-ui"

echo "==> Updating Caddy config..."
scp "$(dirname "$0")/caddy/Caddyfile" "$HOST:/tmp/Caddyfile"
ssh "$HOST" "sudo mv /tmp/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy"

echo "==> Done. UI available at https://cairn.alpina.casa"
