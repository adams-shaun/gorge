#!/usr/bin/env bash
# deploy-demo.sh — stop the running demo gorged, then serve the demo on
# 127.0.0.1:8080: bot-only tables spectate omniscient (every hand shown),
# on-demand play-vs-bot tables stay public so nobody watching a person's
# game sees that person's hand. The separate :8081 omniscient server was
# retired on 2026-09-28; set OMNI_PORT to run a second server again.
#
# 2026-09-29: :8081 serves the same demo with the ManaBrew wire enabled
# (gorged -manabrew, routes on the same origin), for the web client's
# ManaBrew protocol setting (Settings -> Protocol). :8080 stays native-only.
# MB_PORT= (empty) runs no ManaBrew server.
#
# Invoked by `make deploy-demo`, which builds the client and the binary
# first. Run by hand by the operator only: stopping the servers aborts every
# in-flight vs-bot game, so since 2026-09-22 no hook or daemon runs it.
set -euo pipefail

BIN=${BIN:-bin/gorged}
DECKS=${DECKS:-internal/testutil/decks}
# 2026-10-13: 2 tables, the operator's layout: t1 a 1v1 constructed table
# (two bots, SEATS) and t2 a 4-bot commander pod (COMMANDER_SEATS), both
# spectated omniscient ("clairvoyant": every hand shown). On-demand
# play-vs-bot games (-vsbot) are unchanged and come on top of these.
#
# 2026-10-07 (retired): 16 tables, 8 constructed + 8 commander. The demo now serves
# commander games, so the policy must support both formats and 4 seats: the
# old sb-search-lite-atk default is constructed-only/2-seat and
# gorged's validateBotPolicyFlags REJECTS it for lacking commander, which
# would stop the server from listening at all. lethal-pressure is
# constructed+commander and seats 4. TABLES=16 with the FORMATS below
# cycling constructed,commander gives exactly 8 of each (table i takes
# formats[(i-1) mod 2]).
TABLES=${TABLES:-2}
# Seats of a constructed table, and of a commander table (gorged
# -commander-seats). 2026-10-13: 2 and 4 -- a bot-vs-bot constructed table
# beside a 4-player pod. Before that every table seated 4 (2026-10-07).
SEATS=${SEATS:-2}
COMMANDER_SEATS=${COMMANDER_SEATS:-4}
# Wall-clock delay gorged inserts per decision. This has walked 1.5s -> 250ms
# -> 500ms -> 250ms; the user asked for 250ms back on 2026-09-07, so 250ms is
# the ruling and the earlier "tighter than a person can follow" judgement was
# the orchestrator's, not theirs. Kept the default in both places -- here and
# `gorged -pace` -- so "the default pace" means one number. Override for a
# single deploy with `PACE=1.5s make deploy-demo`.
#
# 2026-09-28: 50ms, the operator's call for the single omniscient demo.
PACE=${PACE:-50ms}
# Spectator visibility of the startup (bot-only) tables, and of on-demand
# play-vs-bot tables (-vsbot-spectator). Omniscient never exposes library
# order; it must never be used for a table with a person in it.
SPECTATOR=${SPECTATOR:-omniscient}
VSBOT_SPECTATOR=${VSBOT_SPECTATOR:-public}
# Hosted bot policy (host/bot_policy.go's closed vocabulary) for every
# startup table and for a play-vs-bot game that names none.
#
# 2026-10-13: bot. Of the three commander-capable entries (bot,
# lethal-pressure, cast-profile) it is the strongest: on auto-pay tables
# (-bot-auto-mana, on by default) bot is lethal-pressure plus the
# combat-simulation attacker/blocker, while lethal-pressure plays as the
# plain auto-pay bot. That arm passed the held-out gate at 53.69%
# [52.73, 54.64] Commander and 53.17% [52.68, 53.65] constructed vs the
# plain auto-pay bot (commit 66aa546dd; docs/superpowers/reports/
# 2026-10-09-bot-consolidation-plan.md finding 1). Same cost (<1 ms per
# decision), so the 2026-10-07 load reasoning below still holds.
#
# 2026-10-07 (retired): lethal-pressure. The demo serves 8 constructed and 8 commander
# tables (see TABLES above), and lethal-pressure is constructed+commander and
# seats 4, so it is the only default that satisfies gorged's
# validateBotPolicyFlags for that table mix. It also removes the sb-search
# cost blowup the previous default carried: sb-search-lite-atk spends 219ms
# mean per searched decision (p99 1.1s) on EVERY priority/attackers decision
# of every table, and with several tables searching at once that load froze
# the box (see the retired az-redeal note below); lethal-pressure is the
# cheap manual policy. The search policies remain registered and offered for
# on-demand play-vs-bot games a caller names explicitly.
#
# 2026-09-29 (retired): sb-search-lite-atk (bots/sbsearch), down from
# az-redeal. az-redeal's honest MCTS froze the box when several tables'
# searches piled up at once right at match-end. There is no bail-out yet for
# EITHER search-based policy (tracked: cli-20260929T214831Z-6b1aaa71, a
# wall-clock deadline for the search loop); sb-search-lite-atk is not
# meaningfully safer on cost -- 219ms mean/searched decision vs az-redeal's
# 247ms, same ballpark (docs/superpowers/specs/2026-09-28-spellbench-agent-
# design.md §12.6). Both are Experimental and constructed-only/2-seat-only
# (bots.Info.Formats/MaxSeats), so neither could serve the commander half of
# the demo at all. Before that (2026-09-28): lethal-pressure (AR7).
BOT_POLICY=${BOT_POLICY:-bot}
# false -- the demo serves every policy UNWRAPPED by the bots/sanity
# CommonSense guard. COMMON_SENSE=true opts the `bot` and `sb-tactical`
# seats into the guard (hostile self-removal dropped from the option list,
# the London mulligan answered by botpolicy.MulliganConstructed); measured
# 2026-10-09 on constructed games only (docs/superpowers/reports/2026-10-09-
# common-sense-rules-evaluation.md: `bot` 52.6% [50.9, 54.3], `sb-tactical`
# 51.3% [49.6, 53.0], hostile self-hits 17% -> 0.4%), so it stays opt-in
# until a commander measurement lands (cli-20261013T173521Z-a00fec60).
COMMON_SENSE=${COMMON_SENSE:-false}
# constructed,commander -- table i takes formats[(i-1) mod 2], so TABLES=2
# deals t1 constructed and t2 commander.
FORMATS=${FORMATS:-constructed,commander}
# Deterministic across deploys: the same seed deals the same opening tables,
# so a UI change is the only thing that differs between two screenshots.
SEED=${SEED:-1}
# GOMEMLIMIT for the gorged processes only. The Makefile exports a
# test-oriented `GOMEMLIMIT ?= 5GiB` sized for `go test` package binaries;
# inherited by the server it drove the GC to run continuously with a live
# heap near that ceiling (~84% of the box in gcDrain, measured 2026-09-26).
# A distinct variable name here means the Makefile's export can never
# silently reach the server; a long-running demo wants a higher ceiling.
DEMO_GOMEMLIMIT=${DEMO_GOMEMLIMIT:-8GiB}
# gorged -pprof: Go runtime profiles at /debug/pprof/ on each demo server.
# Off by default. gorged refuses -pprof on a non-loopback listener, and the
# demo binds 127.0.0.1, but anything the box forwards to these ports (a
# tunnel or reverse proxy publishing the demo) forwards /debug/pprof/ with
# it. Set PPROF=true for one deploy to profile it, then redeploy without.
PPROF=${PPROF:-false}

# AZ_C3_NET / AZ_C3_DECKS (both or neither) serve the az-c3 bot: its .dzgw network
# and the directory of the FDN decks it was trained on (docs/superpowers/reports/
# 2026-10-09-az-c3-hosted.md). Set, the demo gains an az-c3 vs az-c3 table —
# bot-only, so setting AZ_C3_NET is the operator's explicit opt-in for
# -allow-decklist-bots (az-c3 reads every seat's real decklist; it never plays
# against a human, and the play-vs-bot deck list no longer offers it).
AZ_C3_NET=${AZ_C3_NET-}
AZ_C3_DECKS=${AZ_C3_DECKS-}
# AZ_C4_NET / AZ_C4_DECKS / AZ_C4_BELIEF (all three or none) serve the az-c4
# bot (docs/superpowers/reports/2026-10-14-az-c4-hosted.md): its .dzgw network,
# the FDN decks it may play (AZ_C3_DECKS's directory may be reused: a shared
# deck is reserved for both) and the belief corpus it guesses its opponent's
# list from; AZ_C4_BELIEF_POOL optionally names a pool.tsv whose train split
# is the belief (the C4 corpus: dzgorge-repro/inputs/decks + pool.tsv).
# az-c4 never reads an opponent's decklist, so it is offered in the
# play-vs-bot picker against humans and needs NO -allow-decklist-bots; it also
# gets its own az-c4 vs az-c4 spectator table.
AZ_C4_NET=${AZ_C4_NET-}
AZ_C4_DECKS=${AZ_C4_DECKS-}
AZ_C4_BELIEF=${AZ_C4_BELIEF-}
AZ_C4_BELIEF_POOL=${AZ_C4_BELIEF_POOL-}
# AZ_C5_NET / AZ_C5_DECKS / AZ_C5_BELIEF (all three or none), and the optional
# AZ_C5_BELIEF_POOL, serve the az-c5 bot exactly as the AZ_C4 variables serve
# az-c4 (docs/superpowers/reports/2026-10-15-az-c5.md): the C5 net, trained
# after the payment-plan fixes, with az-c4's decks and belief. AZ_C5_DECKS may
# be AZ_C4_DECKS's (or AZ_C3_DECKS's) directory: a shared deck is reserved for
# every policy that names it. It gets its own az-c5 vs az-c5 spectator table.
AZ_C5_NET=${AZ_C5_NET-}
AZ_C5_DECKS=${AZ_C5_DECKS-}
AZ_C5_BELIEF=${AZ_C5_BELIEF-}
AZ_C5_BELIEF_POOL=${AZ_C5_BELIEF_POOL-}
# AZ_C6_NET / AZ_C6_DECKS / AZ_C6_BELIEF (all three or none), and the optional
# AZ_C6_BELIEF_POOL, serve the az-c6 bot exactly as the AZ_C5 variables serve
# az-c5: the C6 net (closed-list self-play restarted after the 2026-10-16
# data loss, trained on 17lands FDN decks), with az-c5's config. AZ_C6_DECKS
# may share another az policy's directory. It gets its own az-c6 vs az-c6
# spectator table.
AZ_C6_NET=${AZ_C6_NET-}
AZ_C6_DECKS=${AZ_C6_DECKS-}
AZ_C6_BELIEF=${AZ_C6_BELIEF-}
AZ_C6_BELIEF_POOL=${AZ_C6_BELIEF_POOL-}
PUB_PORT=${PUB_PORT:-8080}
# OMNI_PORT empty (the default) runs no second server.
OMNI_PORT=${OMNI_PORT-}
# 2026-09-29 (later): off by default -- it duplicated every table's az-redeal
# search load onto a second port, and that's the load that froze the box.
# MB_PORT=8081 to bring the ManaBrew wire back once the search cost is tuned.
MB_PORT=${MB_PORT-}
MB_DIR=${MB_DIR:-/tmp/gorge-demo-mb}
MB_LOG=${MB_LOG:-/tmp/gorge-demo-mb.log}
# Ports a previous demo layout served on. The sweep stops a gorged still
# listening there, so the first deploy after a layout change does not leave
# the old server running forever; nothing is started on them.
#
# The set is COMPUTED, not listed: demo-ports.sh unions every port the demo has
# ever bound (an append-only history) with the ones this run binds, so
# emptying a *_PORT variable to retire a listener can never take a previous
# layout's server off the sweep. That bug left the :8081 -manabrew gorged
# standing after 2026-09-29 and tripped the standing_gorged_excess stability
# veto (2026-09-30). RETIRED_PORTS stays as a caller override for a one-off
# port a manual layout used; it is not the mechanism anymore.
RETIRED_PORTS=${RETIRED_PORTS-}
# shellcheck source=scripts/demo-ports.sh
. "$(dirname "${BASH_SOURCE[0]}")/demo-ports.sh"
PUB_DIR=${PUB_DIR:-/tmp/gorge-demo-pub}
OMNI_DIR=${OMNI_DIR:-/tmp/gorge-demo-omni}
PUB_LOG=${PUB_LOG:-/tmp/gorge-demo-pub.log}
OMNI_LOG=${OMNI_LOG:-/tmp/gorge-demo-omni.log}

# Durable card-art cache, OUTSIDE the wiped persistence dirs (task
# fb-20260914T113850Z-682e875e). The `rm -rf` inside start_one stays: a stale
# persistence config resuming with zero-valued fields is a correctness hazard
# (ledger finding cp). The art cache has no such hazard — every cache key is
# hashed under artKeyVersion (cmd/gorged/art.go), so a change in what a fetch
# writes rotates every key at once and stale bytes can never come back
# semantically wrong — which is why the wipe's rationale does NOT extend to
# the art cache, and why it must not be re-added: wiping it used to destroy
# the whole deck pool's art on every merge and leave browsers showing missing
# art for minutes while the cache refilled one paced Scryfall request at a
# time (gorged's startup prewarm refills it from the deck files instead).
# /mnt/sata is the reporter's suggested durable home; it is rw in the
# controller/deploy mount namespace (the server's /proc/<pid>/mounts shows it
# rw) even though an agent worktree jail binds it ro. Both servers share the
# one dir, so each name is fetched from Scryfall once per deploy, not once
# per server (single-flight is per-process, so a first-fetch race between the
# two servers can still fetch a cold name twice — harmless: image and facts
# writers use process-unique staging files and publish only complete artifacts
# with atomic renames). Scryfall pacing IS shared by every lock-aware gorged
# whose BINARY knows the stamp file (artCache.paceWait): the fill below and
# both new servers pace each other, and so do old servers a deploy replaces
# once they run a binary this new. The FIRST deploy after the stamp file's
# introduction overlaps old-binary servers, which pace only themselves; the
# fill's budget bounds that unshared window's duration, not its aggregate
# request rate. From the second deploy on, every lock-aware process sharing
# this dir stays within ~10 req/s.
ART_DIR=${ART_DIR:-/mnt/sata/gorge-data/art}

# Bounds on the pre-start art fill. Art is cosmetic: Scryfall being slow,
# rate-limiting or down must never block or indefinitely delay a deploy, and
# the post-merge hook waits only 600s for the deploy lock before it gives up
# on the NEXT merge's deploy. ART_FILL_BUDGET is gorged's own clean stop
# (it prints the summary); ART_FILL_MAX_FAILURES stops a pass against an
# erroring Scryfall after that many names in a row; ART_FILL_CEILING is the
# hard wall-clock kill in case the binary itself wedges. Build, fill ceiling
# and start together stay well inside the hook's 600s.
ART_FILL_BUDGET=${ART_FILL_BUDGET:-200s}
ART_FILL_MAX_FAILURES=${ART_FILL_MAX_FAILURES:-10}
ART_FILL_CEILING=${ART_FILL_CEILING:-230s}
# The ceiling prefers GNU timeout. uutils coreutils 0.2.2 (this box's
# /usr/bin/timeout) never escalates `-k` to SIGKILL and exits 125 rather than
# 124 on a ceiling (measured 2026-09-14: a child ignoring TERM ran its full 8s
# under uutils, and was killed at 1.3s by gnutimeout, Ubuntu's GNU build).
# That is the fallback's limit: under uutils the ceiling stops WAITING at
# 230s but cannot force-kill a fill wedged past gorged's own budget. Because
# -prewarm-art-only never listens, a later deploy's listener-only sweep cannot
# find it; a TERM-ignoring child can survive indefinitely until manual cleanup.
# With no timeout at all, gorged's own -prewarm-art-budget is the only bound.
TIMEOUT=$(command -v gnutimeout || command -v timeout || true)

AZ_ARGS=()
if [ -n "$AZ_C3_NET$AZ_C3_DECKS" ]; then
	[ -n "$AZ_C3_NET" ] && [ -n "$AZ_C3_DECKS" ] || { echo "deploy-demo: AZ_C3_NET and AZ_C3_DECKS go together" >&2; exit 1; }
	[ -r "$AZ_C3_NET" ] && [ -d "$AZ_C3_DECKS" ] || { echo "deploy-demo: AZ_C3_NET is not a file or AZ_C3_DECKS is not a directory" >&2; exit 1; }
	AZ_ARGS=(-az-c3-net "$AZ_C3_NET" -az-c3-decks "$AZ_C3_DECKS" -allow-decklist-bots)
fi
# closed_az C4 adds the az-c4 flags from the AZ_C4_* variables (C5: az-c5, C6: az-c6).
# No -allow-decklist-bots: the closed-list policies never read a decklist.
closed_az() {
	local v=AZ_$1 p
	p=az-$(printf %s "$1" | tr '[:upper:]' '[:lower:]')
	local net_v=${v}_NET decks_v=${v}_DECKS belief_v=${v}_BELIEF pool_v=${v}_BELIEF_POOL
	local net=${!net_v} decks=${!decks_v} belief=${!belief_v} pool=${!pool_v}
	[ -n "$net$decks$belief$pool" ] || return 0
	[ -n "$net" ] && [ -n "$decks" ] && [ -n "$belief" ] || { echo "deploy-demo: ${v}_NET, ${v}_DECKS and ${v}_BELIEF go together" >&2; exit 1; }
	[ -r "$net" ] && [ -d "$decks" ] && [ -d "$belief" ] || { echo "deploy-demo: ${v}_NET is not a file, or ${v}_DECKS or ${v}_BELIEF is not a directory" >&2; exit 1; }
	[ -z "$pool" ] || [ -r "$pool" ] || { echo "deploy-demo: ${v}_BELIEF_POOL is not a file" >&2; exit 1; }
	AZ_ARGS+=("-$p-net" "$net" "-$p-decks" "$decks" "-$p-belief" "$belief")
	[ -z "$pool" ] || AZ_ARGS+=("-$p-belief-pool" "$pool")
}
closed_az C4
closed_az C5
closed_az C6
# EVENT_DRAFTER (default rating-colour) is what draft-event bot seats draft
# with: net:WEIGHTS.dpkw[@TABLE.json] or ovr:VALUES.json#FIELD (gorged
# -event-drafter, internal/draftpersona).
EVENT_DRAFTER=${EVENT_DRAFTER:-rating-colour}
AZ_ARGS+=(-event-drafter "$EVENT_DRAFTER")
say() { printf 'deploy-demo: %s\n' "$*"; }

# SWEEP=ports (default) stops only the gorged serving the two demo ports —
# on the ADDRESSES this deploy binds (127.0.0.1, plus a wildcard listener,
# which would keep those binds from succeeding). SWEEP=all stops every gorged
# on the box.
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
#
# The port sweep matches the LISTENER ADDRESS, not just the port number:
# "the thing occupying the ports I am about to bind" binds 127.0.0.1:P
# itself, or a wildcard (0.0.0.0/[*]/*:P) that would keep that bind from
# succeeding. A gorged bound to another SPECIFIC address of the same port
# — [::1]:P, say, a probe or a review server — keeps its own bind and its
# own port number to itself; sweeping it killed exactly such a process
# (found by a re-reviewer probing [::1]:8090 while the deploy test ran).
gorged_pids() {
	local filter='LISTEN'
	if [ "$SWEEP" != "all" ]; then
		local ports
		ports=$(demo_sweep_ports "$PUB_PORT" "$OMNI_PORT" "$MB_PORT" $RETIRED_PORTS | paste -sd'|')
		filter="[[:space:]](127\\.0\\.0\\.1|0\\.0\\.0\\.0|\\*|\\[::\\]):($ports)[[:space:]]"
	fi
	# `|| true` is load-bearing, not defensive noise. grep exits 1 when it
	# matches nothing, and with `set -o pipefail` that failure becomes the
	# pipeline's status, which `set -e` then turns into an exit -- from a
	# command substitution, so the script dies at `pids=$(gorged_pids)`
	# BEFORE its first line of output: exit 1, no server, no message.
	# "Nothing is listening" is the ordinary case for a free port, and it is
	# invisible on 8080/8081 precisely because something is always bound
	# there. Found by the distill thread deploying a review server to 8082.
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
	local port=$1 spectator=$2 dir=$3 log=$4 vsbot_spectator=$5
	shift 5
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
	setsid nohup env GOMEMLIMIT="$DEMO_GOMEMLIMIT" "$BIN" \
		-addr "127.0.0.1:$port" \
		-spectator "$spectator" \
		-vsbot-spectator "$vsbot_spectator" \
		-bot-policy "$BOT_POLICY" \
		-bot-common-sense="$COMMON_SENSE" \
		-dir "$dir" \
		-decks "$DECKS" \
		-tables "$TABLES" \
		-seats "$SEATS" \
		-commander-seats "$COMMANDER_SEATS" \
		-pace "$PACE" \
		-format "$FORMATS" \
		-seed "$SEED" \
		-art-dir "$ART_DIR" \
		-pprof="$PPROF" \
		-vsbot \
		-events \
		"$@" \
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

# Fill the card-art cache BEFORE any server is touched, so a deploy normally
# never serves a cold-cache missing-art window and the two servers' startup
# prewarms become no-ops. The cache lives in the durable ART_DIR (outside the
# wiped persistence dirs), and the fill is idempotent: a second run makes zero
# fetches. A genuine Scryfall 404 is a fact (a .miss marker), not a failure.
#
# The fill is BOUNDED (see ART_FILL_* above) and it NEVER aborts the deploy.
# A non-zero exit — failed names, a spent budget, a tripped failure streak or
# the hard ceiling — is reported loudly and the servers start anyway: their
# background prewarm (the same fill loop, unbounded) completes the cache.
# Aborting instead let one flaky name keep new code off the demo, and an
# unbounded 429 storm (about 60s per name) hold the deploy lock for hours
# while later merges' deploys timed out behind it. Rate limiting and 429
# backoff live inside gorged itself (artCache.paceWait / lookupNamed), so this
# can never burst the API.
say "filling card-art cache from $DECKS into $ART_DIR (budget $ART_FILL_BUDGET, ceiling $ART_FILL_CEILING)"
art_rc=0
${TIMEOUT:+"$TIMEOUT" -k 5s "$ART_FILL_CEILING"} "$BIN" -prewarm-art-only \
	-prewarm-art-budget "$ART_FILL_BUDGET" \
	-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES" \
	-decks "$DECKS" -art-dir "$ART_DIR" || art_rc=$?
if [ "$art_rc" -ne 0 ]; then
	say "!!! art fill INCOMPLETE (exit $art_rc; 124/137 = the $ART_FILL_CEILING hard ceiling under GNU timeout; 125 = uutils timeout gave up after TERM and cannot force-kill a wedged fill) — see the summary above; starting the servers anyway, their background prewarm finishes the cache"
fi

if [ -n "$AZ_C3_DECKS" ]; then
	say "filling card-art cache for the az-c3 decks in $AZ_C3_DECKS"
	${TIMEOUT:+"$TIMEOUT" -k 5s "$ART_FILL_CEILING"} "$BIN" -prewarm-art-only \
		-prewarm-art-budget "$ART_FILL_BUDGET" \
		-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES" \
		-decks "$AZ_C3_DECKS" -art-dir "$ART_DIR" || say "!!! az-c3 art fill incomplete (exit $?); the demo still starts"
fi
if [ -n "$AZ_C4_DECKS" ] && [ "$AZ_C4_DECKS" != "$AZ_C3_DECKS" ]; then
	say "filling card-art cache for the az-c4 decks in $AZ_C4_DECKS"
	${TIMEOUT:+"$TIMEOUT" -k 5s "$ART_FILL_CEILING"} "$BIN" -prewarm-art-only \
		-prewarm-art-budget "$ART_FILL_BUDGET" \
		-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES" \
		-decks "$AZ_C4_DECKS" -art-dir "$ART_DIR" || say "!!! az-c4 art fill incomplete (exit $?); the demo still starts"
fi
if [ -n "$AZ_C5_DECKS" ] && [ "$AZ_C5_DECKS" != "$AZ_C3_DECKS" ] && [ "$AZ_C5_DECKS" != "$AZ_C4_DECKS" ]; then
	say "filling card-art cache for the az-c5 decks in $AZ_C5_DECKS"
	${TIMEOUT:+"$TIMEOUT" -k 5s "$ART_FILL_CEILING"} "$BIN" -prewarm-art-only \
		-prewarm-art-budget "$ART_FILL_BUDGET" \
		-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES" \
		-decks "$AZ_C5_DECKS" -art-dir "$ART_DIR" || say "!!! az-c5 art fill incomplete (exit $?); the demo still starts"
fi
if [ -n "$AZ_C6_DECKS" ] && [ "$AZ_C6_DECKS" != "$AZ_C3_DECKS" ] && [ "$AZ_C6_DECKS" != "$AZ_C4_DECKS" ] && [ "$AZ_C6_DECKS" != "$AZ_C5_DECKS" ]; then
	say "filling card-art cache for the az-c6 decks in $AZ_C6_DECKS"
	${TIMEOUT:+"$TIMEOUT" -k 5s "$ART_FILL_CEILING"} "$BIN" -prewarm-art-only \
		-prewarm-art-budget "$ART_FILL_BUDGET" \
		-prewarm-art-max-consecutive-failures "$ART_FILL_MAX_FAILURES" \
		-decks "$AZ_C6_DECKS" -art-dir "$ART_DIR" || say "!!! az-c6 art fill incomplete (exit $?); the demo still starts"
fi
stop_all
start_one "$PUB_PORT" "$SPECTATOR" "$PUB_DIR" "$PUB_LOG" "$VSBOT_SPECTATOR" ${AZ_ARGS[@]+"${AZ_ARGS[@]}"}
if [ -n "$OMNI_PORT" ]; then start_one "$OMNI_PORT" omniscient "$OMNI_DIR" "$OMNI_LOG" public ${AZ_ARGS[@]+"${AZ_ARGS[@]}"}; fi
if [ -n "$MB_PORT" ]; then start_one "$MB_PORT" "$SPECTATOR" "$MB_DIR" "$MB_LOG" "$VSBOT_SPECTATOR" -manabrew; fi
wait_ready "$PUB_PORT"
[ -z "$OMNI_PORT" ] || wait_ready "$OMNI_PORT"
[ -z "$MB_PORT" ] || wait_ready "$MB_PORT"

# Report what each table actually IS, not what the flags asked for. The
# formats above are the request; this line is the server's own answer, and
# it is the thing that regressed silently once already.
say "$(curl -fsS "http://127.0.0.1:$PUB_PORT/api/tables" |
	tr ',' '\n' | grep '"format"' | cut -d'"' -f4 | sort | uniq -c |
	tr '\n' ' ')on :$PUB_PORT"
say "ready — spectator ($SPECTATOR, bots $BOT_POLICY, seats $SEATS/commander $COMMANDER_SEATS, pace $PACE) http://localhost:$PUB_PORT/${OMNI_PORT:+  ·  omniscient http://localhost:$OMNI_PORT/}${MB_PORT:+  ·  manabrew wire http://localhost:$MB_PORT/}"
