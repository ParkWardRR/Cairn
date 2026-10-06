#!/usr/bin/env bash
# Walks every GET route in tests/routes.json against a running web layer and reports
# the status of each. Used by the staging run (web on spare ports against the demo
# tsdb) and by the production cutover check.
#
#   ui/tests/walk-routes.sh [--empty] http://localhost:3100
#
# --empty is for a store with no trips: 400, 404 and 503 (no boot to ask about, nothing to show, no snapshot)
# then count as answered as designed. Against a populated store they are failures.
#
# Exit 0 only if every route answered 2xx. Routes with a parameter other than :bootId
# are skipped (they need a place id the caller owns); POST/PATCH/DELETE are not walked.
set -euo pipefail

EMPTY=0
if [ "${1:-}" = --empty ]; then EMPTY=1; shift; fi
BASE="${1:?usage: walk-routes.sh [--empty] <base-url>}"
BASE="${BASE%/}"
DIR="$(cd "$(dirname "$0")" && pwd)"

boot="$(curl -fsS "$BASE/api/trips" | sed -n 's/.*"boot_id":"\([0-9a-f]*\)".*/\1/p' | head -1)"
[ -n "$boot" ] || echo "note: /api/trips lists no trip, so the :bootId routes are skipped" >&2

ok=0; bad=0; skipped=0
while read -r method path; do
  [ "$method" = GET ] || continue
  case "$path" in
    *:bootId*) [ -n "$boot" ] || { skipped=$((skipped+1)); continue; }; path="${path//:bootId/$boot}" ;;
    *:*) skipped=$((skipped+1)); continue ;;
  esac
  # boot_id is ignored by routes that do not read it
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 60 "$BASE$path?boot_id=$boot" || echo 000)"
  case "$code" in
    2??) ok=$((ok+1)) ;;
    400|404|503) if [ "$EMPTY" = 1 ]; then ok=$((ok+1)); else bad=$((bad+1)); echo "FAIL $code $path"; fi ;;
    *) bad=$((bad+1)); echo "FAIL $code $path" ;;
  esac
done < <(sed -n 's/.*"method": "\([A-Z]*\)", "path": "\([^"]*\)".*/\1 \2/p' "$DIR/routes.json")

echo "$ok ok, $bad failed, $skipped skipped"
[ "$bad" -eq 0 ]
