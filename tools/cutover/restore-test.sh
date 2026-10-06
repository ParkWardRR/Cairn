#!/usr/bin/env bash
# Restores a snapshot (snapshot.sh) into a scratch directory and BOOTS it: the snapshot's own
# store, ingest server and web build, on spare loopback ports, over the snapshot's own data
# and keys. It then asks the restored web layer the same questions as the live one and
# requires the same answers. Live services are not touched.
#
#   restore-test.sh <snapshot-dir>      run ON the host, as a user with sudo
#
# Passing means: the archive is intact, everything needed to run is in it, and it comes up.
set -euo pipefail

snap="${1:?usage: restore-test.sh <snapshot-dir>}"
sudo test -f "$snap/state.tar.gz" || { echo "no state.tar.gz in $snap" >&2; exit 1; }
root="$(mktemp -d /tmp/cairn-restore.XXXXXX)"
sudo chmod 0755 "$root"
pids=()
P_TSDB=18980; P_APP=18944; P_SERVE=18945; P_LOCAL=18946; P_INGEST=18943; P_WEB=18903

cleanup() {
  for p in "${pids[@]:-}"; do [ -n "$p" ] && sudo kill "$p" 2>/dev/null || true; done
  # the backgrounded sudo wrappers do not take their children with them: stop whatever is
  # still listening on this script's spare ports
  for port in $P_TSDB $P_APP $P_SERVE $P_LOCAL $P_INGEST $P_WEB; do
    sudo ss -ltnpH "sport = :$port" 2>/dev/null | grep -o 'pid=[0-9]*' | cut -d= -f2 | sort -u | xargs -r sudo kill 2>/dev/null || true
  done
  sleep 1
  sudo rm -rf "$root"
}
trap cleanup EXIT

echo "==> verifying the archive"
want="$(sudo awk '/state.tar.gz/{print $1; exit}' "$snap/MANIFEST")"
got="$(sudo sha256sum "$snap/state.tar.gz" | awk '{print $1}')"
[ "$want" = "$got" ] || { echo "archive digest differs from the manifest" >&2; exit 1; }
echo "    digest matches"

echo "==> restoring into $root"
sudo tar --xattrs --acls -xpzf "$snap/state.tar.gz" -C "$root"

R="$root"
need=(var/lib/cairn/keystore.json etc/cairn/keystore.master etc/cairn/server.env usr/local/bin/cairn-server usr/local/bin/cairn-tsdb srv/cairn-ui/.output/server/index.mjs)
for f in "${need[@]}"; do sudo test -e "$R/$f" || { echo "the snapshot has no $f" >&2; exit 1; }; done

echo "==> booting the restored stack on spare ports"
sudo mkdir -p "$R/var/lib/cairn-tsdb/sd"; sudo chown -R cairn:cairn "$R/var/lib/cairn-tsdb" 2>/dev/null || true
sudo -u cairn nohup "$R/usr/local/bin/cairn-tsdb" -data "$R/var/lib/cairn" -keystore "$R/var/lib/cairn/keystore.json" \
  -keystore-master "$R/etc/cairn/keystore.master" -sd "$R/var/lib/cairn-tsdb/sd" -addr "127.0.0.1:$P_TSDB" -memory 512MB -watch 5s \
  >"$R/tsdb.log" 2>&1 & pids+=($!)
sudo -u cairn nohup "$R/usr/local/bin/cairn-server" -keystore-master "$R/etc/cairn/keystore.master" \
  -addr "127.0.0.1:$P_INGEST" -data "$R/var/lib/cairn" \
  -tls-cert "$R/etc/cairn/certs/server.pem" -tls-key "$R/etc/cairn/certs/server-key.pem" -tls-client-ca "$R/etc/cairn/certs/ca.pem" \
  -app-addr "127.0.0.1:$P_APP" -app-serve-addr "127.0.0.1:$P_SERVE" -app-snapshot-url "http://127.0.0.1:$P_TSDB" -app-local-addr "127.0.0.1:$P_LOCAL" \
  >"$R/server.log" 2>&1 & pids+=($!)
mkdir -p "$R/webdata"; sudo cp -a "$R/var/lib/cairn-ui/." "$R/webdata/" 2>/dev/null || true; sudo chown -R cairn:cairn "$R/webdata"
( cd "$R/srv/cairn-ui" && sudo -u cairn env NITRO_PORT=$P_WEB NITRO_HOST=127.0.0.1 NUXT_TSDB_URL="http://127.0.0.1:$P_TSDB" \
    NUXT_PLACES_DATA_DIR="$R/webdata" NUXT_PLACES_EXTERNAL=false nohup /usr/bin/node .output/server/index.mjs >"$R/web.log" 2>&1 ) & pids+=($!)

wait_for() { for _ in $(seq 1 120); do curl -fsS -o /dev/null "$1" 2>/dev/null && return 0; sleep 0.5; done; echo "timed out waiting for $1" >&2; for l in tsdb server web; do echo "--- $l.log" >&2; sudo tail -5 "$R/$l.log" >&2 || true; done; return 1; }
wait_for "http://127.0.0.1:$P_TSDB/healthz"
wait_for "http://127.0.0.1:$P_WEB/api/vehicles"

echo "==> the restored web layer against the live one"
norm() { sed -E 's/"(backup_at|exported_at|elapsed_us|observed_at)":[^,}]*/"\1":0/g'; }
fail=0
for p in vehicles dashboard/stats device/tsdb-status trips heatmap; do
  a="$(curl -fsS "http://127.0.0.1:3000/api/$p" | norm || echo LIVE-ERROR)"
  b="$(curl -fsS "http://127.0.0.1:$P_WEB/api/$p" | norm || echo RESTORED-ERROR)"
  if [ "$a" = "$b" ]; then echo "    same   /api/$p"; else echo "    DIFFER /api/$p" >&2; fail=1; fi
done
# saved places come from the restored places database, not the store
a="$(curl -fsS "http://127.0.0.1:3000/api/places/saved/export" | norm)"
b="$(curl -fsS "http://127.0.0.1:$P_WEB/api/places/saved/export" | norm)"
if [ "$a" = "$b" ]; then echo "    same   /api/places/saved/export"; else echo "    DIFFER /api/places/saved/export" >&2; fail=1; fi

[ "$fail" = 0 ] || { echo "the restored stack does not answer like the live one" >&2; exit 1; }
echo "==> restore test passed: the snapshot is complete and boots"
