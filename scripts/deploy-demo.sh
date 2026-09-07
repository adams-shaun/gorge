#!/usr/bin/env bash
# deploy-demo.sh — stop every running gorged, then serve the demo on
# 127.0.0.1:8080 (public spectator) and 127.0.0.1:8081 (omniscient).
#
# Invoked by `make deploy-demo`, which builds the client and the binary
# first. The .githooks/post-merge hook runs that target in the background
# after a merge into main, so the demo is never older than main.
set -euo pipefail

BIN=${BIN:-bin/gorged}
DECKS=${DECKS:-internal/testutil/decks}
TABLES=${TABLES:-4}
SEATS=${SEATS:-4}
# Wall-clock delay gorged inserts per decision. 1.5s was chosen when the
# demo was something you glanced at; watching a game actually play needs it
# an order of magnitude tighter. Override for a single deploy with
# `PACE=1.5s make deploy-demo` -- the flag itself has always been there,
# it was the default that made the demo feel fixed.
PACE=${PACE:-250ms}
# Two Commander tables and two constructed ones, so the overview's
# per-format sections both have something in them. The list is cycled over
# the tables, so this is exactly "half and half" at -tables 4.
FORMATS=${FORMATS:-commander,commander,constructed,constructed}
# Deterministic across deploys: the same seed deals the same opening tables,
# so a UI change is the only thing that differs between two screenshots.
SEED=${SEED:-1}

PUB_PORT=${PUB_PORT:-8080}
OMNI_PORT=${OMNI_PORT:-8081}
PUB_DIR=${PUB_DIR:-/tmp/gorge-demo-pub}
OMNI_DIR=${OMNI_DIR:-/tmp/gorge-demo-omni}

say() { printf 'deploy-demo: %s\n' "$*"; }

# SWEEP=ports (default) stops only the gorged serving the two demo ports.
# SWEEP=all stops every gorged on the box.
#
# The default is narrow on purpose. A wide sweep once killed a task agent's
# own measurement server mid-run: agents are told to serve on 8090-8099 to
# stay clear of the demo, and a deploy that kills everything makes that
# instruction worthless and silently corrupts their results. "Kill the thing
# occupying the ports I am about to bind" is the actual requirement; killing
# every gorged on the machine is a bigger hammer than the job needs.
SWEEP=${SWEEP:-ports}

# gorged_pids lists the pids of the LISTENING gorged this deploy should stop.
#
# Deliberately not `pkill -f gorged` or `pgrep -f`: a bare -f pattern is
# matched against every process's /proc/<pid>/cmdline INCLUDING this
# script's own, which here has killed the caller's shell and orphaned a
# day of work. Sockets are the safe index -- a server that is serving has
# a listening socket -- and /proc/<pid>/comm is an exact process name, not
# a substring of a command line, so nothing else can match it.
gorged_pids() {
	local filter='LISTEN'
	if [ "$SWEEP" != "all" ]; then
		filter="(:$PUB_PORT|:$OMNI_PORT)[[:space:]]"
	fi
	ss -lptn 2>/dev/null |
		grep -E "$filter" |
		grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u |
		while read -r pid; do
			[ -r "/proc/$pid/comm" ] || continue
			[ "$(cat "/proc/$pid/comm")" = "gorged" ] || continue
			echo "$pid"
		done
}

stop_all() {
	local pids
	pids=$(gorged_pids)
	if [ -z "$pids" ]; then
		say "no gorged running"
		return 0
	fi
	say "stopping gorged: $(echo "$pids" | tr '\n' ' ')"
	# SIGTERM first: gorged flushes its persistence directory on the way
	# out, and a half-written match log is what makes the next start
	# resume something odd.
	for pid in $pids; do kill "$pid" 2>/dev/null || true; done
	for _ in $(seq 1 50); do
		[ -z "$(gorged_pids)" ] && return 0
		sleep 0.1
	done
	say "escalating to SIGKILL"
	for pid in $(gorged_pids); do kill -9 "$pid" 2>/dev/null || true; done
	sleep 0.3
}

start_one() {
	local port=$1 spectator=$2 dir=$3 log=$4
	# A FRESH directory every deploy, on purpose. gorged resumes a table
	# set from its persistence dir, and a config written by an older binary
	# comes back with the fields that binary did not have set to their zero
	# values -- Format's zero is "constructed", a real value, so a resumed
	# pre-format data dir silently serves four constructed tables and
	# ignores -format entirely, with nothing in the output to say so.
	# (Ledger finding cp.) The demo is disposable; determinism beats
	# history here.
	rm -rf "$dir"
	# 9>&- CLOSES THE DEPLOY LOCK'S FD IN THE SERVER. The post-merge hook
	# holds its flock on fd 9, and a child inherits every open descriptor --
	# so without this the servers themselves keep the lock file open for
	# their entire life, and the NEXT deploy blocks on a lock held by the
	# processes it is trying to replace. That is a deadlock the flock was
	# meant to prevent: observed as a merge whose deploy sat waiting behind
	# its own predecessor's servers. Harmless when fd 9 is not open.
	setsid nohup "$BIN" \
		-addr "127.0.0.1:$port" \
		-spectator "$spectator" \
		-dir "$dir" \
		-decks "$DECKS" \
		-tables "$TABLES" \
		-seats "$SEATS" \
		-pace "$PACE" \
		-format "$FORMATS" \
		-seed "$SEED" \
		>"$log" 2>&1 </dev/null 9>&- &
	say "started $spectator on 127.0.0.1:$port (log $log)"
}

wait_ready() {
	local port=$1
	for _ in $(seq 1 100); do
		if curl -fsS --max-time 2 "http://127.0.0.1:$port/api/tables" >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	say "TIMED OUT waiting for 127.0.0.1:$port"
	return 1
}

if [ "${1:-}" = "--stop-only" ]; then
	stop_all
	exit 0
fi

[ -x "$BIN" ] || { say "no binary at $BIN (run make deploy-demo, not this script)"; exit 1; }

stop_all
start_one "$PUB_PORT" public "$PUB_DIR" /tmp/gorge-demo-pub.log
start_one "$OMNI_PORT" omniscient "$OMNI_DIR" /tmp/gorge-demo-omni.log
wait_ready "$PUB_PORT"
wait_ready "$OMNI_PORT"

# Report what each table actually IS, not what the flags asked for. The
# formats above are the request; this line is the server's own answer, and
# it is the thing that regressed silently once already.
say "$(curl -fsS "http://127.0.0.1:$PUB_PORT/api/tables" |
	tr ',' '\n' | grep '"format"' | cut -d'"' -f4 | sort | uniq -c |
	tr '\n' ' ')on :$PUB_PORT"
say "ready — spectator http://localhost:$PUB_PORT/  ·  omniscient http://localhost:$OMNI_PORT/"
