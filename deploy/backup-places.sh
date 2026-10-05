#!/usr/bin/env bash
# Pulls a copy of the saved and learned places off the server, so they exist
# somewhere other than the machine that serves them. Safe to run any time and to
# schedule (cron, launchd): it only reads.
#
#   deploy/backup-places.sh [destination]    default: ~/cairn-backups/places
#
# The files contain real locations (home included). Keep them out of the repo.
set -euo pipefail

HOST="${CAIRN_HOST:-alfa@cairn.alpina.casa}"
STATE="/var/lib/cairn-ui"
DEST="${1:-$HOME/cairn-backups/places}"

mkdir -p "$DEST/backups"
# The files belong to the service user, so read them through sudo.
rsync -az --rsync-path="sudo rsync" "$HOST:$STATE/saved-places.json" "$DEST/saved-places.json"
rsync -az --rsync-path="sudo rsync" "$HOST:$STATE/backups/" "$DEST/backups/"
chmod -R go-rwx "$DEST"
echo "saved places copied to $DEST ($(ls "$DEST/backups" | wc -l | tr -d ' ') dated copies)"
