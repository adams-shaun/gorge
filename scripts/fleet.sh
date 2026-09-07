#!/usr/bin/env bash
# fleet.sh — coordination between the two orchestrator threads sharing this
# repo (engine and distill). The protocol it enforces is documented in
# .superpowers/fleet/PROTOCOL.md; this script is the part that is mechanical.
#
# Design rule: every command here is either instant or fails fast. Nothing in
# this script ever blocks one thread waiting on the other, with the single
# deliberate exception of `merge`, whose lock is held for seconds.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")
FLEET="$ROOT/.superpowers/fleet"
SEATS="$FLEET/seats"
TOKENS="$FLEET/tokens"
# Two POOLS, not one cap (user ruling 2026-09-07). The four-seat limit was
# only ever about this box saturating: past four local agents, load-sensitive
# tests fail for reasons unrelated to any diff. A paid seat (codex/Claude) runs
# on someone else's hardware and does not load the box at all, so it cannot be
# what that limit is protecting. Paid seats get their own ceiling for a
# different reason -- they share one ChatGPT plan's rate limit and throttle
# each other and the user's own sessions.
MAX_SEATS=${MAX_SEATS:-4}          # local seats: what the BOX can carry
MAX_PAID=${MAX_PAID:-2}            # paid seats: what the PLAN can carry

# Which thread is running this. Set FLEET_THREAD in the session's environment;
# the file is the fallback so a session that forgets still identifies itself.
THREAD=${FLEET_THREAD:-$(cat "$FLEET/thread" 2>/dev/null || echo engine)}

mkdir -p "$SEATS" "$TOKENS"

die() { echo "fleet: $*" >&2; exit 1; }

# kind_of prints the pool a claimed seat belongs to. A seat directory written
# before pools existed carries no `kind` file; it is local, which is the
# conservative reading (it counts against the scarcer, box-bound pool).
kind_of() { cat "$1/kind" 2>/dev/null || echo local; }

# count_pool counts claimed seats in one pool. Never use `ls | wc -l` for this:
# the two pools now share one directory.
count_pool() {
	local want=$1 n=0 s
	for s in "$SEATS"/*; do
		[ -d "$s" ] || continue
		[ "$(kind_of "$s")" = "$want" ] && n=$((n + 1))
	done
	echo "$n"
}

# --- ports -----------------------------------------------------------------
# Resolved from listening sockets, never from a process-name pattern: a bare
# `pgrep -f` matches this script's own cmdline. comm is the BINARY name, so an
# agent's `gorged-after` build does not read as "gorged" -- which is why the
# port, not the name, is the index.
port_owner() {
	# `|| true`: grep exits 1 on no match, and under `set -o pipefail` +
	# `set -e` that kills the caller from inside a command substitution. A
	# port with nothing on it is the ordinary case here.
	ss -lptn 2>/dev/null | grep -E "127.0.0.1:$1[[:space:]]|\*:$1[[:space:]]" |
		grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u || true
	return 0
}

lane_for() {
	# The SHARED arm MUST come first: rules/heads_test.go is a golden owned by
	# both threads and would otherwise be swallowed by the rules/* pattern in
	# the engine arm. Order in a case statement is the whole classification.
	case "$1" in
		Makefile|AGENTS.md|go.mod|go.sum|rules/heads_test.go|rules/acceptance_test.go) echo SHARED ;;
		rules/*|effects/*|state/*|events/*|cards/*|decision/*|view/*|replay/*|protocol/*|host/*|web/*|cmd/gorged/*|cmd/gentypes/*|scripts/*|.githooks/*) echo engine ;;
		botpolicy/*|seat/*|cmd/botbench/*|internal/testutil/*|distill/*) echo distill ;;
		*) echo unclaimed ;;
	esac
}

# seat_kind says which harness is driving a seat and, for the ones the
# dashboard cannot see, when it last did anything.
#
# The monitor at :8765 globs <repo>/.worktrees/*/.ds4/{transcripts,pi-sessions}
# for its jsonl, so it shows pi and ds4 seats only. A CLAUDE subagent -- which
# is what visual tasks get, deliberately -- writes none of those files and is
# therefore invisible there, permanently and by construction, not because
# anything went wrong. That is worth stating in the one place someone goes to
# ask "where is my agent", rather than leaving them to conclude it died.
seat_kind() {
	local wt="$ROOT/.worktrees/$1"
	[ -d "$wt" ] || { echo "no worktree"; return 0; }
	if compgen -G "$wt/.ds4/pi-sessions/*.jsonl" >/dev/null 2>&1 ||
		compgen -G "$wt/.ds4/transcripts/*.jsonl" >/dev/null 2>&1; then
		echo "pi/ds4 — on the dash :8765"
		return 0
	fi
	# Newest file it has written, excluding the two directories that churn
	# for reasons unrelated to the agent (node_modules is copied wholesale;
	# .git moves on every command run against the worktree).
	local newest age
	newest=$(find "$wt" \( -name node_modules -o -name .git \) -prune -o -type f -printf '%T@\n' 2>/dev/null |
		sort -rn | head -1 | cut -d. -f1)
	if [ -n "$newest" ]; then
		age=$(( ( $(date +%s) - newest ) / 60 ))
		echo "Claude subagent — NOT on the dash; wrote ${age}m ago"
	else
		echo "Claude subagent — NOT on the dash; no writes yet"
	fi
}

cmd_status() {
	echo "fleet status — you are the '$THREAD' thread"
	echo
	echo "main: $(git -C "$ROOT" log --oneline -1 2>/dev/null)"
	local remote
	remote=$(git -C "$ROOT" log --oneline origin/main -1 2>/dev/null || echo '(no origin)')
	echo "origin/main: $remote"
	echo
	echo "== seats (local $(count_pool local)/$MAX_SEATS · paid $(count_pool paid)/$MAX_PAID)"
	if [ -n "$(ls -A "$SEATS" 2>/dev/null)" ]; then
		for s in "$SEATS"/*; do
			local name id
			name=$(basename "$s")
			id=${name#*-}
			printf '  %-22s [%-5s] %-38s %s\n' "$name" "$(kind_of "$s")" "$(seat_kind "$id")" "$(cat "$s/why" 2>/dev/null || echo '')"
		done
	else
		echo "  (none)"
	fi
	echo
	echo "== tokens"
	local any=0
	for t in heads quiet-box merge; do
		if [ -d "$TOKENS/$t" ]; then
			any=1
			printf '  %-10s HELD by %s — %s\n' "$t" \
				"$(cat "$TOKENS/$t/thread" 2>/dev/null)" "$(cat "$TOKENS/$t/why" 2>/dev/null)"
		fi
	done
	[ "$any" = 0 ] && echo "  (all free)"
	echo
	echo "== worktrees"
	git -C "$ROOT" worktree list | tail -n +2 | while read -r path sha branch; do
		printf '  %-14s %s %s\n' "$(basename "$path")" "$sha" "$branch"
	done
	echo
	cmd_ports
	echo
	echo "== the other thread's state file"
	local other="$FLEET/distill.md"
	[ "$THREAD" = distill ] && other="$FLEET/engine.md"
	if [ -f "$other" ]; then
		echo "  $other (updated $(date -r "$other" '+%Y-%m-%d %H:%M'))"
		sed -n '1,12p' "$other" | sed 's/^/  | /'
	else
		echo "  $other does not exist yet"
	fi
}

cmd_ports() {
	echo "== ports (8080-8099)"
	local p pid
	for p in $(seq 8080 8099); do
		for pid in $(port_owner "$p"); do
			printf '  %-6s pid %-8s %-16s %s\n' "$p" "$pid" \
				"$(cat "/proc/$pid/comm" 2>/dev/null || echo '?')" \
				"$(basename "$(readlink "/proc/$pid/cwd" 2>/dev/null || echo '?')")"
		done
	done
	echo "  ranges: 8080-8081 demo (engine, never take) · 8082-8089 distill · 8090-8099 engine agents"
}

# free_port prints the first unused port in this thread's agent range, so a
# brief can name a concrete port instead of "pick one".
cmd_port() {
	local lo=8090 hi=8099
	[ "$THREAD" = distill ] && { lo=8082; hi=8089; }
	local p
	for p in $(seq "$lo" "$hi"); do
		[ -z "$(port_owner "$p")" ] && { echo "$p"; return 0; }
	done
	die "no free port in $lo-$hi for the $THREAD thread"
}

cmd_claim() {
	local id=${1:?usage: fleet.sh claim <task-id> [--paid|--local] [why]}
	shift || true
	# The pool is a flag, not a guess: a dispatch knows which seat it is about
	# to spend, and inferring it later from a transcript is how a paid run gets
	# miscounted as free.
	local kind=local
	case "${1:-}" in
		--paid)  kind=paid;  shift ;;
		--local) kind=local; shift ;;
	esac
	local cap=$MAX_SEATS
	[ "$kind" = paid ] && cap=$MAX_PAID
	local n
	n=$(count_pool "$kind")
	local seat="$SEATS/$THREAD-$id"
	[ -d "$seat" ] && die "seat $THREAD-$id already claimed"
	# mkdir is the atomic primitive: two threads racing for the last seat
	# cannot both succeed. The count is checked first and re-checked after,
	# because the check itself is not atomic with the mkdir.
	if [ "$n" -ge "$cap" ]; then
		if [ "$kind" = paid ]; then
			die "all $cap PAID seats are claimed — wait, or use a local seat ($(count_pool local)/$MAX_SEATS in use)"
		fi
		die "all $cap LOCAL seats are claimed — use a paid seat ($(count_pool paid)/$MAX_PAID in use) or a Claude subagent rather than waiting"
	fi
	mkdir "$seat" || die "seat $THREAD-$id already claimed"
	# Write the kind BEFORE re-counting, or the re-count cannot see this seat's
	# own pool and every racing claim reads as local.
	echo "$kind" > "$seat/kind"
	n=$(count_pool "$kind")
	if [ "$n" -gt "$cap" ]; then
		rm -rf "$seat"
		die "lost the race for the last $kind seat — use the other pool or a Claude subagent"
	fi
	{ echo "${*:-}"; } > "$seat/why"
	date -Iseconds > "$seat/since"
	echo "fleet: claimed $kind seat $THREAD-$id ($n/$cap $kind; local $(count_pool local)/$MAX_SEATS, paid $(count_pool paid)/$MAX_PAID)"
}

cmd_release() {
	local id=${1:?usage: fleet.sh release <task-id>}
	local seat="$SEATS/$THREAD-$id"
	[ -d "$seat" ] || die "no seat $THREAD-$id to release"
	local kind
	kind=$(kind_of "$seat")
	rm -rf "$seat"
	echo "fleet: released $kind seat $THREAD-$id (local $(count_pool local)/$MAX_SEATS, paid $(count_pool paid)/$MAX_PAID)"
}

cmd_token() {
	local name=${1:?usage: fleet.sh token <heads|quiet-box|merge> <acquire|release|status> [why]}
	local action=${2:-status}
	shift 2 || true
	local dir="$TOKENS/$name"
	case "$action" in
		acquire)
			if ! mkdir "$dir" 2>/dev/null; then
				echo "fleet: '$name' is HELD by $(cat "$dir/thread" 2>/dev/null) — $(cat "$dir/why" 2>/dev/null)" >&2
				echo "fleet: do NOT wait. Ship everything the token does not guard and report BLOCKED on the rest." >&2
				exit 1
			fi
			echo "$THREAD" > "$dir/thread"
			{ echo "${*:-}"; } > "$dir/why"
			date -Iseconds > "$dir/since"
			echo "fleet: acquired '$name'"
			;;
		release)
			[ -d "$dir" ] || die "'$name' is not held"
			local holder
			holder=$(cat "$dir/thread" 2>/dev/null || echo '?')
			[ "$holder" = "$THREAD" ] || die "'$name' is held by $holder, not you — say so in your inbox instead of taking it"
			rm -rf "$dir"
			echo "fleet: released '$name'"
			;;
		status)
			if [ -d "$dir" ]; then
				echo "'$name' HELD by $(cat "$dir/thread" 2>/dev/null) since $(cat "$dir/since" 2>/dev/null) — $(cat "$dir/why" 2>/dev/null)"
			else
				echo "'$name' is free"
			fi
			;;
		*) die "unknown token action '$action'" ;;
	esac
}

cmd_lane() {
	[ $# -gt 0 ] || die "usage: fleet.sh lane <path>..."
	local mine=0 theirs=0 shared=0
	local p l
	for p in "$@"; do
		l=$(lane_for "$p")
		printf '  %-10s %s\n' "$l" "$p"
		case "$l" in
			SHARED) shared=1 ;;
			"$THREAD") mine=1 ;;
			unclaimed) ;;
			*) theirs=1 ;;
		esac
	done
	echo
	[ "$theirs" = 1 ] && echo "CROSSES THE OTHER THREAD'S LANE: land those files as their own small commit on main FIRST, then note it in your inbox (PROTOCOL.md §2)."
	[ "$shared" = 1 ] && echo "TOUCHES SHARED FILES: heads/ratchet goldens need the 'heads' token."
	[ "$theirs$shared" = "00" ] && echo "Entirely within the $THREAD lane — no coordination needed."
	return 0
}

# merge runs a command under the merge lock. The lock is held for the command's
# duration only, so it must be the merge itself, never a whole task.
cmd_merge() {
	[ "${1:-}" = "--" ] && shift
	[ $# -gt 0 ] || die "usage: fleet.sh merge -- <command>"
	# 9>&- in any server the command starts: a child inherits the lock fd and
	# would hold it for its lifetime. That deadlock has happened here.
	flock -w 300 9 || die "timed out waiting for the merge lock — check 'fleet.sh token merge status'"
	echo "fleet: merge lock held by $THREAD"
	"$@"
} 9>"$FLEET/merge.lock"

case "${1:-status}" in
	status)  cmd_status ;;
	ports)   cmd_ports ;;
	port)    cmd_port ;;
	claim)   shift; cmd_claim "$@" ;;
	release) shift; cmd_release "$@" ;;
	token)   shift; cmd_token "$@" ;;
	lane)    shift; cmd_lane "$@" ;;
	merge)   shift; cmd_merge "$@" ;;
	*) die "unknown command '${1}' (status|ports|port|claim|release|token|lane|merge)" ;;
esac
