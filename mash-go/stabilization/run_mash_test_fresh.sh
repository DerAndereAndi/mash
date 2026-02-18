#!/usr/bin/env bash
set -euo pipefail

# Runs exactly one mash-test invocation against a freshly reset mash-device.
# Usage:
#   ./stabilization/run_mash_test_fresh.sh <mash-test args...>
#
# Example:
#   ./stabilization/run_mash_test_fresh.sh \
#     -target localhost:8443 -mode device -setup-code 20220211 \
#     -enable-key deadbeefdeadbeefdeadbeefdeadbeef -json -filter "TC-PROTO*"

if [[ $# -eq 0 ]]; then
  echo "usage: $0 <mash-test args...>" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MASH_DEVICE_TYPE="${MASH_DEVICE_TYPE:-evse}"
MASH_DEVICE_PORT="${MASH_DEVICE_PORT:-8443}"
MASH_SETUP_CODE="${MASH_SETUP_CODE:-20220211}"
MASH_DISCRIMINATOR="${MASH_DISCRIMINATOR:-1234}"
MASH_ENABLE_KEY="${MASH_ENABLE_KEY:-deadbeefdeadbeefdeadbeefdeadbeef}"
MASH_DEVICE_LOG_LEVEL="${MASH_DEVICE_LOG_LEVEL:-debug}"

STAMP="$(date +%Y%m%d-%H%M%S)"
STATE_DIR="${MASH_STATE_DIR:-/tmp/mash-device-state-${STAMP}-fresh}"
LOG_DIR="${MASH_LOG_DIR:-$ROOT_DIR/stabilization/phase1-runs}"
mkdir -p "$LOG_DIR"
DEVICE_LOG="${MASH_DEVICE_LOG:-$LOG_DIR/device-${STAMP}.log}"

cleanup() {
  if [[ -n "${DEVICE_PID:-}" ]]; then
    kill "$DEVICE_PID" >/dev/null 2>&1 || true
    wait "$DEVICE_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "${DEVICE_BIN:-}" ]]; then
    rm -f "$DEVICE_BIN" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Ensure the target port is not already occupied by a stale process.
if lsof -nP -iTCP:"$MASH_DEVICE_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "error: tcp port $MASH_DEVICE_PORT is already in use; stop stale mash-device processes first" >&2
  lsof -nP -iTCP:"$MASH_DEVICE_PORT" -sTCP:LISTEN >&2 || true
  exit 1
fi

DEVICE_BIN="$(mktemp /tmp/mash-device-wrapper-bin-XXXXXX)"
go build -o "$DEVICE_BIN" ./cmd/mash-device

"$DEVICE_BIN" \
  -type "$MASH_DEVICE_TYPE" \
  -port "$MASH_DEVICE_PORT" \
  -setup-code "$MASH_SETUP_CODE" \
  -discriminator "$MASH_DISCRIMINATOR" \
  -simulate \
  -enable-key "$MASH_ENABLE_KEY" \
  -state-dir "$STATE_DIR" \
  -reset \
  -log-level "$MASH_DEVICE_LOG_LEVEL" \
  >"$DEVICE_LOG" 2>&1 &
DEVICE_PID=$!

# Wait until THIS mash-device process starts listening.
for _ in $(seq 1 40); do
  if ! kill -0 "$DEVICE_PID" >/dev/null 2>&1; then
    echo "error: mash-device exited before becoming ready; see $DEVICE_LOG" >&2
    exit 1
  fi
  if lsof -nP -a -p "$DEVICE_PID" -iTCP:"$MASH_DEVICE_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done

if ! lsof -nP -a -p "$DEVICE_PID" -iTCP:"$MASH_DEVICE_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "error: mash-device did not become ready on port $MASH_DEVICE_PORT; see $DEVICE_LOG" >&2
  exit 1
fi

go run ./cmd/mash-test "$@"
