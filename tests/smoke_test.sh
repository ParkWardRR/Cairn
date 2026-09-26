#!/usr/bin/env bash
# Cairn smoke test -- quick pipeline sanity check.
#
# Usage:
#   ./tests/smoke_test.sh                           # default: localhost:8443
#   ./tests/smoke_test.sh http://cairn.example.lan:8443
#
# Exits 0 on success, 1 on failure.

set -euo pipefail

SERVER="${1:-http://localhost:8443}"
API="${SERVER}/api/v1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TMPDIR_SMOKE=""

cleanup() {
    if [ -n "$TMPDIR_SMOKE" ] && [ -d "$TMPDIR_SMOKE" ]; then
        rm -rf "$TMPDIR_SMOKE"
    fi
}
trap cleanup EXIT

pass() { printf "  PASS  %s\n" "$1"; }
fail() { printf "  FAIL  %s: %s\n" "$1" "$2"; exit 1; }

echo "Cairn Smoke Test"
echo "Server: $SERVER"
echo "============================================================"

# -----------------------------------------------------------------------
# 1. Health check
# -----------------------------------------------------------------------
STEP="health check"
HEALTH=$(curl -sf --max-time 5 "$API/health" 2>/dev/null) || fail "$STEP" "server not reachable"
STATUS=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
if [ "$STATUS" != "ok" ]; then
    fail "$STEP" "status=$STATUS (expected ok)"
fi
pass "$STEP"

# -----------------------------------------------------------------------
# 2. Generate 1 trip
# -----------------------------------------------------------------------
STEP="generate trip"
TMPDIR_SMOKE=$(mktemp -d -t cairn_smoke_XXXXXX)
python3 "$PROJECT_ROOT/zig/simulator/simulate_device.py" \
    --trips 1 \
    --seed 42 \
    --output "$TMPDIR_SMOKE" \
    2>/dev/null || fail "$STEP" "simulator failed"

# Find the generated trip directory
TRIP_DIR=$(find "$TMPDIR_SMOKE" -name "manifest.json" -maxdepth 2 -exec dirname {} \; | head -1)
if [ -z "$TRIP_DIR" ] || [ ! -f "$TRIP_DIR/manifest.json" ]; then
    fail "$STEP" "no trip bundle generated"
fi
TRIP_ID=$(python3 -c "import json; print(json.load(open('$TRIP_DIR/manifest.json'))['trip_id'])")
pass "$STEP (trip_id=$TRIP_ID)"

# -----------------------------------------------------------------------
# 3. Upload the trip
# -----------------------------------------------------------------------
STEP="upload trip"
SAMPLES="$TRIP_DIR/samples.bin"
CONTENT_HASH=$(python3 -c "import hashlib; print(hashlib.sha256(open('$SAMPLES','rb').read()).hexdigest())")
FILE_SIZE=$(wc -c < "$SAMPLES" | tr -d ' ')

# 3a. Init
INIT_RESP=$(curl -sf --max-time 10 \
    -X POST "$API/upload/init" \
    -H "Content-Type: application/json" \
    -d "{\"device_id\":\"cairn-simulator-01\",\"trip_id\":\"$TRIP_ID\",\"content_hash\":\"$CONTENT_HASH\",\"size\":$FILE_SIZE}") \
    || fail "$STEP" "init request failed"

UPLOAD_ID=$(echo "$INIT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['upload_id'])")
RESUME_OFFSET=$(echo "$INIT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('resume_offset',0))")

if [ "$RESUME_OFFSET" = "-1" ]; then
    pass "$STEP (already uploaded, upload_id=$UPLOAD_ID)"
else
    # 3b. Upload in chunks
    OFFSET=${RESUME_OFFSET}
    CHUNK_SIZE=4096
    while [ "$OFFSET" -lt "$FILE_SIZE" ]; do
        END=$((OFFSET + CHUNK_SIZE))
        if [ "$END" -gt "$FILE_SIZE" ]; then
            END=$FILE_SIZE
        fi
        LEN=$((END - OFFSET))

        # Extract chunk using dd
        CHUNK_FILE="$TMPDIR_SMOKE/chunk.bin"
        dd if="$SAMPLES" bs=1 skip="$OFFSET" count="$LEN" of="$CHUNK_FILE" 2>/dev/null

        curl -sf --max-time 10 \
            -X PUT "$API/upload/$UPLOAD_ID/chunk" \
            -H "Content-Type: application/octet-stream" \
            -H "X-Upload-Offset: $OFFSET" \
            --data-binary "@$CHUNK_FILE" \
            >/dev/null || fail "$STEP" "chunk upload failed at offset $OFFSET"

        OFFSET=$END
    done

    # 3c. Finalize
    FIN_RESP=$(curl -sf --max-time 10 \
        -X POST "$API/upload/$UPLOAD_ID/finalize" \
        -H "Content-Type: application/json" \
        -d "{\"content_hash\":\"$CONTENT_HASH\"}") \
        || fail "$STEP" "finalize request failed"

    RECEIPT_ID=$(echo "$FIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('receipt_id',''))")
    pass "$STEP (upload_id=$UPLOAD_ID, receipt=$RECEIPT_ID)"
fi

# -----------------------------------------------------------------------
# 4. Verify it landed
# -----------------------------------------------------------------------
STEP="verify upload"

# Re-init with same hash -- should get resume_offset=-1 (completed)
VERIFY_RESP=$(curl -sf --max-time 10 \
    -X POST "$API/upload/init" \
    -H "Content-Type: application/json" \
    -d "{\"device_id\":\"cairn-simulator-01\",\"trip_id\":\"$TRIP_ID\",\"content_hash\":\"$CONTENT_HASH\",\"size\":$FILE_SIZE}") \
    || fail "$STEP" "verification init request failed"

VERIFY_OFFSET=$(echo "$VERIFY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('resume_offset',0))")
if [ "$VERIFY_OFFSET" != "-1" ]; then
    fail "$STEP" "upload not marked completed (resume_offset=$VERIFY_OFFSET, expected -1)"
fi

# Also check device status
DEV_STATUS=$(curl -sf --max-time 10 "$API/devices/cairn-simulator-01/status" 2>/dev/null) \
    || fail "$STEP" "device status check failed"
pass "$STEP (upload confirmed completed)"

# -----------------------------------------------------------------------
# Done
# -----------------------------------------------------------------------
echo "============================================================"
echo "Smoke test PASSED"
exit 0
