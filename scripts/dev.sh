#!/usr/bin/env bash
# Development supervisor: keeps the latest build of ImageOrganizer running.
#
#  - Go sources are fingerprinted every second; on change the binary is rebuilt
#    and, only if the build succeeds, the server is restarted. A broken build
#    leaves the previous server running.
#  - UI files are served straight from web/ (-dev), so browsers hot-swap CSS
#    or reload on JS/HTML changes without any rebuild.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LISTEN="${IO_DEV_LISTEN:-0.0.0.0:8420}"
BIN_DIR="${IO_DEV_BIN_DIR:-${XDG_RUNTIME_DIR:-/tmp}/imageorganizer-dev}"
BIN="$BIN_DIR/imageorganizer"
POLL="${IO_DEV_POLL:-1}"
EXTRA_ARGS=(${IO_DEV_ARGS:-})

mkdir -p "$BIN_DIR"
cd "$ROOT"

log() { printf '[dev %s] %s\n' "$(date +%H:%M:%S)" "$*"; }

fingerprint() {
  find . -path ./web/vendor -prune -o \( -name '*.go' -o -name go.mod -o -name go.sum \) -type f -printf '%p %T@ %s\n' 2>/dev/null |
    sort | sha1sum | cut -d' ' -f1
}

build() {
  local start=$(date +%s%N)
  if go build -o "$BIN.new" . 2>"$BIN_DIR/build.log"; then
    mv -f "$BIN.new" "$BIN"
    log "build ok in $(( ($(date +%s%N) - start) / 1000000 ))ms"
    return 0
  fi
  log "build FAILED; keeping previous server running:"
  sed 's/^/    /' "$BIN_DIR/build.log"
  return 1
}

child=""
start_server() {
  "$BIN" -listen "$LISTEN" -dev "$ROOT/web" "${EXTRA_ARGS[@]}" &
  child=$!
  log "server pid=$child on $LISTEN"
}

# The server shuts down gracefully on SIGTERM within its own 8s grace period;
# give it a little longer, then SIGKILL.
STOP_TIMEOUT="${IO_DEV_STOP_TIMEOUT:-12}"

stop_server() {
  [[ -n "$child" ]] || return 0
  if kill -0 "$child" 2>/dev/null; then
    local start=$(date +%s%N) i
    kill -TERM "$child" 2>/dev/null || true
    for (( i = 0; i < STOP_TIMEOUT * 10; i++ )); do
      kill -0 "$child" 2>/dev/null || break
      sleep 0.1
    done
    if kill -0 "$child" 2>/dev/null; then
      log "server pid=$child ignored SIGTERM for ${STOP_TIMEOUT}s; killing"
      kill -KILL "$child" 2>/dev/null || true
    else
      log "server pid=$child stopped in $(( ($(date +%s%N) - start) / 1000000 ))ms"
    fi
  fi
  wait "$child" 2>/dev/null || true
  child=""
}

trap 'stop_server; exit 0' INT TERM

until build; do sleep 3; done
start_server
last="$(fingerprint)"

while true; do
  sleep "$POLL"
  # Restart if the server died on its own (crash), with a small backoff.
  if [[ -n "$child" ]] && ! kill -0 "$child" 2>/dev/null; then
    wait "$child" 2>/dev/null || true
    log "server exited unexpectedly; restarting"
    sleep 1
    start_server
  fi
  now="$(fingerprint)"
  [[ "$now" == "$last" ]] && continue
  # Let editors finish writing a batch of files before building.
  sleep 0.5
  now="$(fingerprint)"
  last="$now"
  log "change detected, rebuilding"
  if build; then
    stop_server
    start_server
  fi
done
