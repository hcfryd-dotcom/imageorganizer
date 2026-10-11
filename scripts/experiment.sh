#!/usr/bin/env bash
# Experiment: a copy of the live library in its own data directory, run on
# another port, where who is who starts over (see -start-over). The live
# library is never touched: its database is copied through the running
# server's snapshot, and the caches (thumbnails, face crops, previews,
# models) are shared read-mostly through symlinks. Folders aren't scanned
# and the analysis stages start paused: the copy reuses the live analysis.
#
#   scripts/experiment.sh           run it (a fresh copy the first time)
#   scripts/experiment.sh --fresh   copy the live library again, and start over
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIVE="${IO_LIVE_DATA:-$HOME/.local/share/imageorganizer}"
EXP="${IO_EXP_DATA:-$HOME/.local/share/imageorganizer-exp}"
LIVE_URL="${IO_LIVE_URL:-http://127.0.0.1:8420}"
LISTEN="${IO_EXP_LISTEN:-0.0.0.0:8421}"
ORT="${IO_ORT:-$HOME/.local/lib/onnxruntime-gpu-1.29.0/lib/libonnxruntime.so}"
BIN_DIR="${XDG_RUNTIME_DIR:-/tmp}/imageorganizer-exp"
BIN="$BIN_DIR/imageorganizer"

log() { printf '[exp %s] %s\n' "$(date +%H:%M:%S)" "$*"; }

start=()
if [[ "${1:-}" == "--fresh" || ! -f "$EXP/library.bolt" ]]; then
  mkdir -p -m 700 "$EXP"
  log "copying the live library from $LIVE_URL"
  curl -sf -o "$EXP/library.bolt.new" "$LIVE_URL/debug/db"
  chmod 600 "$EXP/library.bolt.new" # it holds the passwords of network folders
  mv -f "$EXP/library.bolt.new" "$EXP/library.bolt"
  for d in thumbs faces previews models; do
    ln -sfn "$LIVE/$d" "$EXP/$d"
  done
  start=(-start-over)
fi

mkdir -p "$BIN_DIR"
(cd "$ROOT" && go build -o "$BIN" .)
log "running on $LISTEN with data in $EXP ${start[*]:-}"
exec "$BIN" -listen "$LISTEN" -data "$EXP" -dev "$ROOT/web" -onnxruntime "$ORT" \
  -no-scan -paused import,age,look,embed "${start[@]}"
