#!/usr/bin/env bash
# Launches the demo server and the autonomous defect-pipeline daemon, then
# supervises both: if either dies, it is restarted. Intended to run
# indefinitely.
#
# Usage:
#   scripts/start-autonomous.sh              # start the supervisor in the background (idempotent)
#   scripts/start-autonomous.sh run          # run the supervisor in the foreground (service managers)
#   scripts/start-autonomous.sh stop         # stop the supervisor, then the daemon, cleanly
#   scripts/start-autonomous.sh status       # report what's running
#
# `start` returns immediately and is safe to call any number of times, from
# anywhere: exactly one supervisor and one daemon ever run, because both are
# guarded by kernel file locks (.ds4/orchestrator/{supervisor,daemon}.lock).
#
# Why locks and not PID files: on 2026-09-14 this script was launched five times
# (once from a login shell, four times by a Codex session inside its sandbox).
# The old `start` never returned, so the calling tool call timed out and was
# retried. Each copy judged the daemon dead by `kill -0` on a PID written by a
# copy in a different PID namespace, which it could not see, and started another.
# 49 daemons ended up dispatching and merging from the same state. A flock is
# held on the file itself, so it is honoured across PID namespaces.
#
# The daemon's OWN kill-switch (pause without stopping this supervisor) is
# `touch .ds4/orchestrator/pause` / `rm .ds4/orchestrator/pause`: the daemon
# keeps polling but takes no dispatch/merge action while that file exists.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
REPO="$(pwd)"
STATE="$REPO/.ds4/orchestrator"
mkdir -p "$STATE"

SUPERVISOR_LOCK="$STATE/supervisor.lock"
DAEMON_LOCK="$STATE/daemon.lock"        # taken by orchestrator.daemon itself
DAEMON_PID_FILE="$STATE/daemon.pid"     # informational only; never used for liveness
SUPERVISOR_LOG="$STATE/supervisor.log"
DEMO_PORTS="8080 8081"

log() { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$SUPERVISOR_LOG"; echo "$*"; }

# A lock is held iff we cannot take it ourselves right now.
lock_held() { ! flock -n "$1" true 2>/dev/null; }
supervisor_alive() { lock_held "$SUPERVISOR_LOCK"; }
daemon_alive() { lock_held "$DAEMON_LOCK"; }

# The demo counts as up when any of its ports accepts a connection. A TCP connect
# works from any PID namespace; `ss -p` could not see the servers' PIDs from a
# sandbox, so every sandboxed copy re-ran `make deploy-demo` every 30 seconds.
demo_up() {
  local p
  for p in $DEMO_PORTS; do
    (exec 3<>"/dev/tcp/127.0.0.1/$p") 2>/dev/null && return 0
  done
  return 1
}

# The supervisor must run somewhere that can write the Go build caches: its
# daemon's gates build and test Go, and deploy-demo builds gorged. A read-only
# sandbox (Codex runs commands under bwrap with $HOME read-only) can otherwise
# win the lock and run a pipeline whose every build fails. Prints the first
# unwritable directory, if any.
unwritable_dir() {
  local d
  for d in "$STATE" "$(go env GOCACHE 2>/dev/null)" "$(go env GOMODCACHE 2>/dev/null)"; do
    [ -n "$d" ] || continue
    mkdir -p "$d" 2>/dev/null
    if ( : >"$d/.write-probe.$$" ) 2>/dev/null; then
      rm -f "$d/.write-probe.$$"
    else
      echo "$d"
      return 0
    fi
  done
  return 1
}

refuse_if_sandboxed() {
  local bad
  if bad=$(unwritable_dir); then
    echo "start-autonomous: $bad is not writable -- this looks like a read-only sandbox." >&2
    echo "start-autonomous: run it from a normal login shell (or a service manager) instead." >&2
    exit 1
  fi
}

start_demo_if_down() {
  # Deliberately does NOT deploy: the demo is the operator's to (re)start by
  # hand with `make deploy-demo` (a deploy aborts every in-flight vs-bot
  # game). This only reports, so a missing demo is visible in the log.
  if ! demo_up; then
    log "demo server down; start it by hand: make deploy-demo"
  fi
}

start_daemon() {
  log "starting orchestrator daemon"
  setsid nohup env PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl run "$REPO" >>"$STATE/daemon-stdout.log" 2>&1 </dev/null &
  echo $! >"$DAEMON_PID_FILE"
  disown
  # Wait for it to take its lock before the next liveness check, so a slow
  # import can never read as "dead" and trigger a second launch.
  local _
  for _ in $(seq 1 30); do
    daemon_alive && return 0
    sleep 1
  done
  log "WARNING: daemon did not take $DAEMON_LOCK within 30s (see daemon-stdout.log)"
}

print_status() {
  if supervisor_alive; then echo "supervisor: running"; else echo "supervisor: not running"; fi
  if daemon_alive; then
    echo "daemon: running (last launched pid $(cat "$DAEMON_PID_FILE" 2>/dev/null || echo '?'))"
  else
    echo "daemon: not running"
  fi
  if demo_up; then echo "demo: running"; else echo "demo: not running"; fi
  if [ -f "$STATE/pause" ]; then
    echo "PAUSED (rm $STATE/pause to resume dispatch)"
  fi
}

cmd="${1:-start}"

case "$cmd" in
  start)
    if supervisor_alive; then
      echo "supervisor already running"
      print_status
      exit 0
    fi
    refuse_if_sandboxed
    setsid nohup "$0" run >>"$STATE/supervisor-stdout.log" 2>&1 </dev/null &
    disown
    for _ in $(seq 1 20); do
      supervisor_alive && break
      sleep 0.5
    done
    print_status
    exit 0
    ;;
  run)
    exec 9>"$SUPERVISOR_LOCK"
    if ! flock -n 9; then
      echo "supervisor already running"
      exit 0
    fi
    refuse_if_sandboxed
    ;;
  stop)
    # The supervisor first: stopping only the daemon lets the supervisor restart
    # it within 30 seconds. pkill runs in this PID namespace, so run `stop` from
    # the same kind of shell that started the supervisor.
    pkill -TERM -f 'start-autonomous[.]sh run' 2>/dev/null
    for _ in $(seq 1 20); do
      supervisor_alive || break
      sleep 0.5
    done
    if daemon_alive; then
      log "stopping daemon"
      for p in $(ps -eo pid=,args= | awk -v r="$REPO" '$2=="python3" && $3=="-m" && $4=="agentctl" && $5=="run" && $6==r {print $1}'); do kill -TERM "$p"; done
      echo "waiting for the daemon to finish its current tick..."
      for _ in $(seq 1 180); do
        daemon_alive || break
        sleep 1
      done
    fi
    rm -f "$DAEMON_PID_FILE"
    print_status
    echo "(demo server left running -- stop it separately if you want it down too)"
    exit 0
    ;;
  status)
    print_status
    exit 0
    ;;
  *)
    echo "usage: $0 [start|run|stop|status]" >&2
    exit 2
    ;;
esac

# --- run: the supervisor loop, holding $SUPERVISOR_LOCK on fd 9 --------------
log "supervisor starting (repo=$REPO)"
trap 'log "supervisor received stop signal"; exit 0' TERM INT

while true; do
  start_demo_if_down
  if ! daemon_alive; then
    log "daemon not running, starting it"
    start_daemon
  fi
  # Backgrounded so the TERM trap runs immediately instead of after the sleep.
  sleep 30 &
  wait $!
done
