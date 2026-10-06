#!/usr/bin/env bash
# Cold rollback snapshot of the whole Cairn stack on its host. Run ON the host as a user
# with sudo. Services are stopped for the few seconds the copy takes (nothing is being
# ingested during a cutover) and a trap restarts them whatever happens.
#
#   snapshot.sh [parent-dir]        default /var/backups; prints the snapshot directory
#
# Captured: every writable store (/var/lib/cairn: keystore, counters, vehicles, devices,
# clients, receipts; /var/lib/cairn-ui: saved and learned places and the lookup cache;
# /var/lib/cairn-tsdb), every config (/etc/cairn including secrets, the systemd units and
# drop-ins, /etc/caddy), every build (the binaries and their .prev, the web build), and the
# source trees the previous deploys were made from. The directory is 0700 root: it holds
# the keystore master key.
#
# A snapshot you have not restored is a hope. restore-test.sh restores this one.
set -euo pipefail

parent="${1:-/var/backups}"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
snap="$parent/cairn-cutover-$ts"
sudo install -d -m 0700 "$snap"

units=(cairn-ui cairn-server cairn-tsdb)
was_active=()
for u in "${units[@]}"; do systemctl is-active --quiet "$u" && was_active+=("$u") || true; done

restart() {
  # dependency order: store, then ingest/API, then the web layer
  for u in cairn-tsdb cairn-server cairn-ui; do
    for a in "${was_active[@]:-}"; do [ "$a" = "$u" ] && sudo systemctl start "$u"; done
  done
}
trap restart EXIT

echo "==> stopping: ${was_active[*]:-nothing was running}" >&2
for u in "${units[@]}"; do sudo systemctl stop "$u" 2>/dev/null || true; done

# what exists, relative to /, so the archive restores in place
cd /
include=()
for p in var/lib/cairn var/lib/cairn-ui var/lib/cairn-tsdb etc/cairn etc/caddy srv/cairn-ui \
         etc/systemd/system/cairn-*.service etc/systemd/system/cairn-*.service.d \
         usr/local/bin/cairn-* home/*/cairn-v3; do
  for m in $p; do [ -e "$m" ] && include+=("$m"); done
done

echo "==> archiving ${#include[@]} paths" >&2
sudo tar --xattrs --acls -czpf "$snap/state.tar.gz" "${include[@]}"

{
  echo "snapshot   $snap"
  echo "taken      $ts (UTC)"
  echo "host       $(hostname -s)"
  echo "kernel     $(uname -r)"
  echo "node       $(node -v 2>/dev/null || echo none)"
  echo "units      active before: ${was_active[*]:-none}"
  echo
  echo "sha256 of the archive:"
  sudo sha256sum "$snap/state.tar.gz"
  echo
  echo "sha256 of every build:"
  sudo sh -c 'sha256sum /usr/local/bin/cairn-*'
  [ -d /srv/cairn-ui/.output ] && sudo sh -c 'cd /srv/cairn-ui/.output && find . -type f | sort | xargs sha256sum | sha256sum | sed "s/-/the web build (.output) as one digest/"'
  echo
  echo "archive contents (top level):"
  sudo tar -tzf "$snap/state.tar.gz" | awk -F/ '{print $1"/"$2"/"$3}' | sort -u
} | sudo tee "$snap/MANIFEST" >/dev/null
sudo chmod 0600 "$snap/MANIFEST" "$snap/state.tar.gz"

echo "$snap"
