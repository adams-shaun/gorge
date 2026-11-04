#!/usr/bin/env bash
# deploy-demo.sh — stop the running demo gorged, then serve the demo on
# 127.0.0.1:$PUB_PORT: bot-only tables spectate omniscient (every hand shown),
# on-demand play-vs-bot tables stay public so nobody watching a person's game
# sees that person's hand. Set OMNI_PORT to run a second, omniscient server.
# The public server also serves the ManaBrew wire (gorged -manabrew); its
# routes share the server's tables, so it adds no table and no search load
# until a seat connects over it.
#
#   scripts/deploy-demo.sh [--build | --stop-only | --prewarm-only | --dry-run]
#
#   (none)          deploy BIN (building it first, without the web client,
#                   when it does not exist)
#   --build         refuse a gorge checkout with no draft tree, build the web
#                   client and BIN from GORGE_ROOT, then deploy (make deploy-demo)
#   --stop-only     stop the demo's servers and start nothing (make stop-demo)
#   --prewarm-only  fill the card-art cache from DECKS, strict and unbounded,
#                   and exit with the fill's status (make prewarm-art)
#   --dry-run       print the resolved configuration and every command the
#                   deploy would run; build, stop and start nothing
#
# Configuration: every variable below reads the environment first, then
# deploy/demo.env (untracked; deploy/demo.env.example documents each one,
# DEMO_ENV names another file), then the default written here. The defaults
# are a generic single-box demo; the box's own ports, directories and network
# paths belong in deploy/demo.env.
#
# Stopping the servers aborts every in-flight vs-bot game, so nothing runs this
# automatically: whoever owns the demo runs it by hand.
set -euo pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)

# DEMO_VARS is every variable deploy/demo.env may set. The environment wins
# over the file: a variable already set (even to empty) when the script starts
# keeps its value, so `PACE=1s make deploy-demo` overrides the file for one
# deploy. TestDemoEnvExampleDocumentsEveryVariable keeps the example in step.
DEMO_VARS=(
	GORGE_ROOT BIN CARDS DECKS FEEDBACK
	PUB_PORT OMNI_PORT RETIRED_PORTS DEMO_PORT_HISTORY SWEEP
	PUB_DIR OMNI_DIR PUB_LOG OMNI_LOG
	TABLES SEATS COMMANDER_SEATS FORMATS PACE SEED
	SPECTATOR VSBOT_SPECTATOR BOT_POLICY COMMON_SENSE
	PAIRED_BOT PAIRED_BOT_SIMS PAIRED_BOT_SEARCHES
	DEMO_GOMEMLIMIT PPROF
	ART_DIR ART_FILL_BUDGET ART_FILL_MAX_FAILURES ART_FILL_CEILING
)
DEMO_ENV=${DEMO_ENV:-$SCRIPT_ROOT/deploy/demo.env}
if [ -f "$DEMO_ENV" ]; then
	declare -A _env_wins=()
	for _v in "${DEMO_VARS[@]}"; do
		if [ -n "${!_v+set}" ]; then _env_wins[$_v]=${!_v}; fi
	done
	# shellcheck source=/dev/null
	. "$DEMO_ENV"
	for _v in "${!_env_wins[@]}"; do printf -v "$_v" '%s' "${_env_wins[$_v]}"; done
	unset _v _env_wins
fi

# GORGE_ROOT is the gorge checkout the demo builds and serves from: this
# script's own checkout unless set. GORGE_MAIN is that checkout's main
# worktree (its git common dir), where the gitignored feedback reports live.
GORGE_ROOT=${GORGE_ROOT:-$SCRIPT_ROOT}
if _common=$(git -C "$GORGE_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null); then
	GORGE_MAIN=$(dirname "$_common")
else
	GORGE_MAIN=$GORGE_ROOT
fi
unset _common
BIN=${BIN:-$GORGE_ROOT/bin/gorged}
DECKS=${DECKS:-$GORGE_ROOT/testkit/testutil/decks}
# gorged's -cards and -feedback defaults are relative to the working
# directory, so they are pinned to the checkout.
CARDS=${CARDS:-$GORGE_ROOT/.cards}
FEEDBACK=${FEEDBACK:-$GORGE_MAIN/feedback}

# Two startup tables: t1 a two-bot constructed table, t2 a four-bot Commander
# pod, both spectated omniscient. Table i takes FORMATS[(i-1) mod n].
# On-demand play-vs-bot games (-vsbot) come on top of these.
TABLES=${TABLES:-2}
SEATS=${SEATS:-2}
COMMANDER_SEATS=${COMMANDER_SEATS:-4}
FORMATS=${FORMATS:-constructed,commander}
# Wall-clock delay gorged inserts per bot decision.
PACE=${PACE:-50ms}
# The same seed deals the same opening tables on every deploy, so a UI change
# is the only thing that differs between two screenshots.
SEED=${SEED:-1}
# Spectator visibility of the startup (bot-only) tables and of play-vs-bot
# tables. Omniscient never exposes library order, and must never be used for a
# table with a person in it.
SPECTATOR=${SPECTATOR:-omniscient}
VSBOT_SPECTATOR=${VSBOT_SPECTATOR:-public}
# The hosted policy of every startup table and of a play-vs-bot game that names
# none. A Commander table needs a policy that plays Commander with four seats
# (gorged refuses to start otherwise), which rules out the constructed-only
# search policies; they stay offered for play-vs-bot games that name them.
BOT_POLICY=${BOT_POLICY:-bot}
# true wraps the bot and sb-tactical seats in the bots/sanity common-sense
# guard. Off until it is measured on Commander games.
COMMON_SENSE=${COMMON_SENSE:-false}
# The paired bot (gorged -paired-bot): a person playing alone against bots
# gets a bot beside them that weighs each of their decisions and marks its
# pick, from what that seat can see. az-closed is the search that needs no
# trained network; off turns it off. PAIRED_BOT_SIMS is its simulations per
# decision, PAIRED_BOT_SEARCHES how many run at once across the server (the
# rest wait), so advice never takes more than that many cores.
PAIRED_BOT=${PAIRED_BOT:-az-closed}
PAIRED_BOT_SIMS=${PAIRED_BOT_SIMS:-100}
PAIRED_BOT_SEARCHES=${PAIRED_BOT_SEARCHES:-2}
# GOMEMLIMIT for the servers only, under its own name so a test-oriented
# GOMEMLIMIT in the caller's environment never reaches a long-running server
# (one sized for test binaries kept its GC running continuously).
DEMO_GOMEMLIMIT=${DEMO_GOMEMLIMIT:-8GiB}
# gorged -pprof: runtime profiles at /debug/pprof/. gorged refuses it on a
# non-loopback listener, but anything forwarding the demo's port forwards
# /debug/pprof/ with it: set it for one deploy, then redeploy without.
PPROF=${PPROF:-false}

# PUB_PORT defaults to gorged's own -addr port. OMNI_PORT empty runs no second
# server. RETIRED_PORTS adds one-off ports to the sweep; DEMO_PORT_HISTORY
# (scripts/demo-ports.sh) is the box's append-only history of demo ports.
PUB_PORT=${PUB_PORT:-8080}
OMNI_PORT=${OMNI_PORT-}
RETIRED_PORTS=${RETIRED_PORTS-}
# shellcheck source=scripts/demo-ports.sh
. "$SCRIPT_ROOT/scripts/demo-ports.sh"
# Persistence directories, wiped on every deploy (start_one), and logs.
PUB_DIR=${PUB_DIR:-/tmp/gorge-demo-pub}
OMNI_DIR=${OMNI_DIR:-/tmp/gorge-demo-omni}
PUB_LOG=${PUB_LOG:-/tmp/gorge-demo-pub.log}
OMNI_LOG=${OMNI_LOG:-/tmp/gorge-demo-omni.log}

# The durable card-art cache, outside the wiped persistence directories.
# Wiping it with them used to leave browsers showing missing art for minutes
# while the cache refilled one paced Scryfall request at a time. Stale bytes
# cannot come back wrong: every cache key is hashed under artKeyVersion
# (server/gorged/art.go). Both servers share it, and every gorged sharing it
# paces its Scryfall requests together.
ART_DIR=${ART_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/gorge-demo/art}
# Bounds on the pre-start art fill. Art is cosmetic: a slow, rate-limiting or
# absent Scryfall must never block a deploy. ART_FILL_BUDGET is gorged's own
# clean stop (it prints the summary); ART_FILL_MAX_FAILURES stops a pass after
# that many failed names in a row; ART_FILL_CEILING is the hard wall-clock
# kill in case the binary itself wedges.
ART_FILL_BUDGET=${ART_FILL_BUDGET:-200s}
ART_FILL_MAX_FAILURES=${ART_FILL_MAX_FAILURES:-10}
ART_FILL_CEILING=${ART_FILL_CEILING:-230s}
# The ceiling prefers GNU timeout: uutils' timeout never escalates -k to
# SIGKILL and exits 125 rather than 124. Under it the ceiling stops waiting but
# cannot kill a fill wedged past gorged's own budget. With no timeout at all,
# -prewarm-art-budget is the only bound.
TIMEOUT=$(command -v gnutimeout || command -v timeout || true)

# SWEEP=ports (default) stops only the gorged listening on the ports this
# deploy binds or has ever bound, on the addresses it binds; SWEEP=all stops
# every listening gorged on the box. The default is narrow on purpose: other
# work on the box serves its own gorged (on 8090-8099 by convention), and a
# wide sweep has killed such a server mid-measurement.
SWEEP=${SWEEP:-ports}

MODE=deploy
case "${1:-}" in
"") ;;
--build) MODE=build ;;
--stop-only) MODE=stop ;;
--prewarm-only) MODE=prewarm ;;
--dry-run) MODE=dry ;;
*)
	echo "usage: $0 [--build | --stop-only | --prewarm-only | --dry-run]" >&2
	exit 2
	;;
esac

say() { printf 'deploy-demo: %s\n' "$*"; }
# show prints a command as it would run, quoted.
show() {
	printf '+'
	printf ' %q' "$@"
	printf '\n'
}

# gorged_pids lists the pids of the LISTENING gorged this deploy should stop.
#
# Never `pkill -f gorged` or `pgrep -f`: a bare -f pattern is matched against
# every process's /proc/<pid>/cmdline, this script's own and its caller's
# included, and has killed the caller's shell. Sockets are the safe index (a
# server that is serving has a listening socket), and /proc/<pid>/comm is an
# exact process name, not a substring of a command line.
#
# The port sweep matches the listener ADDRESS as well as the port: the thing
# occupying a port this deploy binds is bound to 127.0.0.1:P itself, or to a
# wildcard (0.0.0.0, [::], *) that would keep that bind from succeeding. A
# gorged bound to another specific address of the same port ([::1]:P, a probe
# or a review server) keeps its own bind and is left alone.
gorged_pids() {
	local filter='LISTEN'
	if [ "$SWEEP" != "all" ]; then
		local ports
		# RETIRED_PORTS is a space-separated list.
		# shellcheck disable=SC2086
		ports=$(demo_sweep_ports "$PUB_PORT" "$OMNI_PORT" $RETIRED_PORTS | paste -sd'|')
		filter="[[:space:]](127\\.0\\.0\\.1|0\\.0\\.0\\.0|\\*|\\[::\\]):($ports)[[:space:]]"
	fi
	# `|| true` is load-bearing: grep exits 1 when nothing listens, which
	# pipefail would turn into the pipeline's status and set -e into an exit
	# from the command substitution, before the script prints anything.
	ss -lptn 2>/dev/null |
		grep -E "$filter" |
		grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u |
		while read -r pid; do
			[ -r "/proc/$pid/comm" ] || continue
			[ "$(cat "/proc/$pid/comm")" = "gorged" ] || continue
			echo "$pid"
		done || true
	return 0
}

stop_all() {
	local pids
	pids=$(gorged_pids)
	if [ -z "$pids" ]; then
		say "no gorged running"
		return 0
	fi
	if [ "$MODE" = dry ]; then
		say "would stop gorged: $(echo "$pids" | tr '\n' ' ')"
		return 0
	fi
	say "stopping gorged: $(echo "$pids" | tr '\n' ' ')"
	# SIGTERM first: gorged flushes its persistence directory on the way out,
	# and a half-written match log is what makes the next start resume
	# something odd.
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
	local port=$1 spectator=$2 dir=$3 log=$4 vsbot_spectator=$5
	shift 5
	local cmd=(env GOMEMLIMIT="$DEMO_GOMEMLIMIT" "$BIN"
		-addr "127.0.0.1:$port"
		-spectator "$spectator"
		-vsbot-spectator "$vsbot_spectator"
		-bot-policy "$BOT_POLICY"
		-bot-common-sense="$COMMON_SENSE"
		-paired-bot "$PAIRED_BOT"
		-paired-bot-sims "$PAIRED_BOT_SIMS"
		-paired-bot-searches "$PAIRED_BOT_SEARCHES"
		-dir "$dir"
		-cards "$CARDS"
		-feedback "$FEEDBACK"
		-decks "$DECKS"
		-tables "$TABLES"
		-seats "$SEATS"
		-commander-seats "$COMMANDER_SEATS"
		-pace "$PACE"
		-format "$FORMATS"
		-seed "$SEED"
		-art-dir "$ART_DIR"
		-pprof="$PPROF"
		-vsbot
		-events
		"$@")
	if [ "$MODE" = dry ]; then
		show rm -rf "$dir"
		show "${cmd[@]}"
		return 0
	fi
	# A fresh directory every deploy, on purpose: gorged resumes a table set
	# from its persistence directory, and a config written by an older binary
	# comes back with the fields that binary lacked at their zero values
	# (Format's zero is constructed, a real value), silently.
	rm -rf "$dir"
	# 9>&- closes a deploy lock's descriptor in the server: a caller holding a
	# flock on fd 9 would otherwise leave the servers holding it for life, and
	# the next deploy would block on a lock held by the processes it replaces.
	setsid nohup "${cmd[@]}" >"$log" 2>&1 </dev/null 9>&- &
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

# fill_art <decks-dir> fills the art cache for one deck directory within the
# budget and the ceiling; it returns the fill's status.
fill_art() {
	local cmd=("$BIN" -prewarm-art-only
		-prewarm-art-budget "$ART_FILL_BUDGET"
		-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES"
		-decks "$1" -art-dir "$ART_DIR")
	if [ -n "$TIMEOUT" ]; then cmd=("$TIMEOUT" -k 5s "$ART_FILL_CEILING" "${cmd[@]}"); fi
	if [ "$MODE" = dry ]; then
		show "${cmd[@]}"
		return 0
	fi
	"${cmd[@]}"
}

# build_bin builds BIN from GORGE_ROOT. With --build it first refuses a
# checkout without the draft tree (gorged links draft events, so the build
# would fail later and less clearly) and builds the web client the binary
# embeds: a web-only change touches no .go file, so the binary is rebuilt
# every time.
build_bin() {
	local bin_dir bin_abs
	bin_dir=$(dirname "$BIN")
	if [ "$MODE" = build ] || [ "$MODE" = dry ]; then
		test -f "$GORGE_ROOT/draft/gorgedevents/plugin.go" || {
			echo "deploy-demo: gorge checkout $GORGE_ROOT has no draft/gorgedevents: it cannot build the demo's gorged" >&2
			exit 1
		}
	fi
	if [ "$MODE" = dry ]; then
		show make -C "$GORGE_ROOT" web
		show env CGO_ENABLED=0 go build -C "$GORGE_ROOT" -trimpath -o "$BIN" ./cmd/gorged
		return 0
	fi
	if [ "$MODE" = build ]; then
		make -C "$GORGE_ROOT" web
	fi
	mkdir -p "$bin_dir"
	bin_abs=$(cd "$bin_dir" && pwd)/$(basename "$BIN")
	say "building gorged from $GORGE_ROOT into $bin_abs"
	CGO_ENABLED=0 go build -C "$GORGE_ROOT" -trimpath -o "$bin_abs" ./cmd/gorged
}

case "$MODE" in
stop)
	stop_all
	exit 0
	;;
prewarm)
	[ -x "$BIN" ] || build_bin
	# By hand this fill is strict and unbounded: no budget, no streak limit,
	# and the fill's own exit status (zero fetches when the cache is complete).
	exec "$BIN" -prewarm-art-only -decks "$DECKS" -art-dir "$ART_DIR"
	;;
dry)
	say "configuration (DEMO_ENV $DEMO_ENV$([ -f "$DEMO_ENV" ] || echo ', absent')):"
	for _v in "${DEMO_VARS[@]}"; do printf '  %s=%q\n' "$_v" "${!_v}"; done
	build_bin
	;;
build)
	build_bin
	;;
deploy)
	if [ ! -x "$BIN" ]; then
		say "no binary at $BIN; building it without rebuilding the web client (make deploy-demo builds both)"
		build_bin
	fi
	;;
esac

# Fill the card-art cache BEFORE any server is touched, so a deploy normally
# never serves a cold-cache window and the servers' own startup prewarms are
# no-ops. A genuine Scryfall 404 is a fact (a .miss marker), not a failure.
#
# The fill is bounded and NEVER aborts the deploy: a non-zero exit (failed
# names, a spent budget, a tripped failure streak or the hard ceiling) is
# reported loudly and the servers start anyway; their background prewarm
# finishes the cache. Aborting instead let one flaky name keep new code off
# the demo. Rate limiting and 429 backoff live inside gorged, so this can
# never burst the API.
say "filling card-art cache from $DECKS into $ART_DIR (budget $ART_FILL_BUDGET, ceiling $ART_FILL_CEILING)"
art_rc=0
fill_art "$DECKS" || art_rc=$?
if [ "$art_rc" -ne 0 ]; then
	say "!!! art fill INCOMPLETE (exit $art_rc; 124/137 = the $ART_FILL_CEILING hard ceiling under GNU timeout; 125 = uutils timeout gave up after TERM and cannot force-kill a wedged fill) — see the summary above; starting the servers anyway, their background prewarm finishes the cache"
fi
stop_all
# No -manabrew-think: it sets the think timeout of every human seat on the
# server, native ones included.
start_one "$PUB_PORT" "$SPECTATOR" "$PUB_DIR" "$PUB_LOG" "$VSBOT_SPECTATOR" -manabrew
if [ -n "$OMNI_PORT" ]; then start_one "$OMNI_PORT" omniscient "$OMNI_DIR" "$OMNI_LOG" public; fi
if [ "$MODE" = dry ]; then
	say "dry run: nothing was built, stopped or started"
	exit 0
fi
wait_ready "$PUB_PORT"
[ -z "$OMNI_PORT" ] || wait_ready "$OMNI_PORT"

# Report what each table actually IS, not what the flags asked for: the
# formats above are the request, this line is the server's own answer.
say "$(curl -fsS "http://127.0.0.1:$PUB_PORT/api/tables" |
	tr ',' '\n' | grep '"format"' | cut -d'"' -f4 | sort | uniq -c |
	tr '\n' ' ')on :$PUB_PORT"
say "ready — spectator ($SPECTATOR, bots $BOT_POLICY, paired bot $PAIRED_BOT, seats $SEATS/commander $COMMANDER_SEATS, pace $PACE) http://localhost:$PUB_PORT/${OMNI_PORT:+  ·  omniscient http://localhost:$OMNI_PORT/}"
