#!/usr/bin/env bash
# Launches the demo server and the autonomous defect-pipeline daemon, then
# supervises both: if either dies, it is restarted. Intended to run
# indefinitely (nohup'd from a login session, or under a real service
# manager) rather than to be watched.
#
# Usage:
#   scripts/start-autonomous.sh              # start (or resume) the fleet
#   scripts/start-autonomous.sh stop         # stop both, cleanly
#   scripts/start-autonomous.sh status       # report what's running
#
# The daemon's OWN kill-switch (pause without stopping this supervisor) is
# `touch .ds4/orchestrator/pause` / `rm .ds4/orchestrator/pause`: the daemon
# keeps polling but takes no dispatch/merge action while that file exists.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
REPO="$(pwd)"
STATE="$REPO/.ds4/orchestrator"
mkdir -p "$STATE"

DAEMON_PID_FILE="$STATE/daemon.pid"
SUPERVISOR_LOG="$STATE/supervisor.log"

log() { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$SUPERVISOR_LOG"; echo "$*"; }

demo_pids() {
  ss -lptnH 2>/dev/null | /usr/bin/grep -E ':808[01] ' | /usr/bin/grep -oP 'pid=\K[0-9]+' | sort -u
}

start_demo_if_down() {
  if [ -z "$(demo_pids)" ]; then
    log "demo server down, starting it"
    make deploy-demo >>"$STATE/demo-deploy.log" 2>&1
  fi
}

daemon_alive() {
  [ -f "$DAEMON_PID_FILE" ] && kill -0 "$(cat "$DAEMON_PID_FILE")" 2>/dev/null
}

start_daemon() {
  log "starting orchestrator daemon"
  setsid nohup python3 -m orchestrator.daemon >>"$STATE/daemon-stdout.log" 2>&1 &
  echo $! >"$DAEMON_PID_FILE"
  disown
}

cmd="${1:-start}"

case "$cmd" in
  stop)
    if daemon_alive; then
      pid="$(cat "$DAEMON_PID_FILE")"
      log "stopping daemon pid $pid"
      kill -TERM "$pid" 2>/dev/null
    fi
    rm -f "$DAEMON_PID_FILE"
    echo "daemon stopped (demo server left running -- stop it separately if you want it down too)"
    exit 0
    ;;
  status)
    if daemon_alive; then
      echo "daemon: running (pid $(cat "$DAEMON_PID_FILE"))"
    else
      echo "daemon: not running"
    fi
    if [ -n "$(demo_pids)" ]; then
      echo "demo: running (pids $(demo_pids | tr '\n' ' '))"
    else
      echo "demo: not running"
    fi
    if [ -f "$STATE/pause" ]; then
      echo "PAUSED (rm $STATE/pause to resume dispatch)"
    fi
    exit 0
    ;;
  start) ;;
  *)
    echo "usage: $0 [start|stop|status]" >&2
    exit 2
    ;;
esac

log "supervisor starting (repo=$REPO)"
start_demo_if_down
if ! daemon_alive; then
  start_daemon
fi

trap 'log "supervisor received stop signal"; exit 0' TERM INT

while true; do
  start_demo_if_down
  if ! daemon_alive; then
    log "daemon not running, restarting"
    start_daemon
  fi
  sleep 30
done
