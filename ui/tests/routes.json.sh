#!/bin/sh
# Regenerates tests/routes.json from server/api. tests/routes.test.ts fails when the
# two disagree, so adding or removing an endpoint means running this and committing.
set -eu
cd "$(dirname "$0")/.."
{
  echo '['
  find server/api -type f -name '*.ts' | sort | while read -r f; do
    rel=${f#server/api/}
    method=$(echo "$rel" | sed -E 's/.*\.(get|post|put|patch|delete)\.ts$/\1/' | tr a-z A-Z)
    p=$(echo "$rel" | sed -E 's/\.(get|post|put|patch|delete)\.ts$//; s#/index$##; s/\[([^]]+)\]/:\1/g')
    printf '  {"method": "%s", "path": "/api/%s"},\n' "$method" "$p"
  done | sed '$ s/,$//'
  echo ']'
} > tests/routes.json
