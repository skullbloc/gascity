#!/usr/bin/env bash
# test-dolt.sh — lifecycle for a shared dolt server used by session-miner
# test cities.
#
# Why this exists: gc's default is to spawn a per-city dolt server, which
# leaves orphans lying around during iterative testing. This script
# manages ONE dolt server on a known port that multiple test cities can
# point at (via `[dolt] host/port` in their city.toml).
#
# Usage:
#   test-dolt.sh start    # start the server if not already running
#   test-dolt.sh stop     # stop the server if running
#   test-dolt.sh status   # print pid + port + listening check
#
# Environment:
#   GC_TEST_DOLT_PORT   default 13307 — change if deacon reports a conflict
#   GC_TEST_DOLT_HOME   default ~/.session-miner-test-dolt — where state lives

set -euo pipefail

: "${GC_TEST_DOLT_PORT:=13307}"
: "${GC_TEST_DOLT_HOME:=$HOME/.session-miner-test-dolt}"

PIDFILE="$GC_TEST_DOLT_HOME/dolt.pid"
LOGFILE="$GC_TEST_DOLT_HOME/dolt.log"
DATADIR="$GC_TEST_DOLT_HOME/data"

die() { echo "test-dolt: $*" >&2; exit 1; }
command -v dolt >/dev/null || die "dolt not on PATH"

cmd="${1:-}"

is_running() {
  [ -f "$PIDFILE" ] || return 1
  pid=$(cat "$PIDFILE" 2>/dev/null)
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

case "$cmd" in
  start)
    # Never bind to the gt town's main dolt port; confirmed with deacon
    # that earlier test runs on :3307 briefly displaced gt's dolt and the
    # gt daemon had to auto-recover. Belt-and-suspenders safety.
    if [ "$GC_TEST_DOLT_PORT" = "3307" ]; then
      die "refusing to bind :3307 (reserved for gt main dolt). Use a high port, e.g. 13307."
    fi
    mkdir -p "$DATADIR"
    if is_running; then
      echo "test-dolt: already running, pid $(cat "$PIDFILE") port $GC_TEST_DOLT_PORT"
      exit 0
    fi
    if ss -tlnp 2>/dev/null | awk '{print $4}' | grep -qE ":${GC_TEST_DOLT_PORT}\$"; then
      die "port $GC_TEST_DOLT_PORT already in use by something else — pick a different GC_TEST_DOLT_PORT"
    fi
    cd "$DATADIR"
    # First-run init: dolt needs a database to exist before accepting conns
    # against it. We init a harmless bootstrap db; actual test cities will
    # create their own databases via `bd init` inside the server.
    if [ ! -d "$DATADIR/bootstrap" ]; then
      mkdir -p bootstrap
      cd bootstrap && dolt init --fun > /dev/null && cd ..
    fi
    nohup dolt sql-server \
      --host 127.0.0.1 --port "$GC_TEST_DOLT_PORT" \
      --log-level warning \
      > "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 1
    if is_running; then
      echo "test-dolt: started pid $(cat "$PIDFILE") on port $GC_TEST_DOLT_PORT"
      echo "test-dolt: log at $LOGFILE"
    else
      tail -20 "$LOGFILE" >&2
      die "startup failed; see log above"
    fi
    ;;
  stop)
    if ! is_running; then
      echo "test-dolt: not running"
      rm -f "$PIDFILE"
      exit 0
    fi
    pid=$(cat "$PIDFILE")
    kill "$pid"
    for _ in 1 2 3 4 5; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid"
    fi
    rm -f "$PIDFILE"
    echo "test-dolt: stopped"
    ;;
  status)
    if is_running; then
      pid=$(cat "$PIDFILE")
      echo "test-dolt: running pid $pid port $GC_TEST_DOLT_PORT"
      ss -tlnp 2>/dev/null | awk -v p=":$GC_TEST_DOLT_PORT" '$4 ~ p {print "  listener: " $4}'
    else
      echo "test-dolt: not running"
    fi
    ;;
  *)
    die "usage: $0 {start|stop|status}"
    ;;
esac
