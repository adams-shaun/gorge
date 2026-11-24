#!/usr/bin/env bash
# test-remote-box.sh probe|run|finish|cleanup <bare-repo> [<run> <mode>]: the half of scripts/test-remote.sh that
# runs ON THE GATE BOX. test-remote.sh feeds it to `bash -s` over ssh, so the box needs no copy of it.
#
#   probe    <repo>              janitor (scratch worktrees and refs older than a day), then one line:
#                                probe idle=<cores> free=<free run slots> slots=<run slots> lock=<corpus key: cards.lock hash + script listing hash>
#   run      <repo> <run> <mode> the run slot's worktree at refs/tt/<run>/head (+ <repo>/../tt-wt/<run>.patch),
#                                the same scripts/test-targets.sh <mode> in a systemd scope whose CPUWeight is below
#                                the landing gate's (rgate.scope runs at 100). It streams the output, writes
#                                <repo>/../tt-wt/<run>.status and exits 0; 91 = no free run slot; 92 = the box could
#                                not set the run up (the status file says why). Neither is a test result.
#   finish   <repo> <run>        tar (status + .ds4/scratch/test) to stdout, then cleanup
#   cleanup  <repo> <run>        stop the scope, remove the run's results, the refs, the patch and the status
#                                (never the slot worktree: the next run in that slot reuses it)
#
# Settings arrive as TT_* environment variables (set by the client from ~/.config/gorge/test-targets.env):
# TT_MAXRUNS (the run slots; empty means max(2, nproc/6), so the slots follow the box's cores), TT_WEIGHT, TT_CPU
# (percent, default 1200), TT_MEM, TT_PROCS (default 8), TT_CARDS (the corpus on this box), TT_GOROOT, TT_NICE
# (the run's nice level, default 5: below the landing gate's 0 as well as under its CPUWeight).
# Layout: <repo>/../tt-wt/.slot<i>.wt, one worktree per run slot, reused; <repo>/../tt-wt/<run>/.ds4/scratch/test, a
# run's results, copied out of the slot before its lock is released; <repo>/../gocache (the landing gate's build cache, kept warm),
# <repo>/../pools (deck pools, when present).
set -uo pipefail
cmd=${1:-}
repo=${2:-}
run=${3:-}
mode=${4:-}
# corpus_key DIR: the corpus's identity, "<cards.lock sha256 prefix><script listing sha256 prefix>" (32 hex), or
# nothing when DIR has no cards.lock. The lock alone is not enough: an extract or rsync without --delete left a box
# 366 scripts richer than its lock said, and a fetch inside a symlinked overlay left a lock over another ref's scripts
# (engine/cards ListingOf is the same listing hash; test-remote-box.sh carries a copy of this function, one script
# being fed to the box over ssh, and TestTestTargetsRemoteCorpusKeyAgrees keeps the two equal).
corpus_key() {
	local l s
	l=$(sha256sum "$1/cards.lock" 2>/dev/null | cut -c1-16)
	[ -n "$l" ] || return 1
	s=$(cd "$1" && find -L cardsfolder tokenscripts -type f -name '*.txt' 2>/dev/null | LC_ALL=C sort | sha256sum | cut -c1-16)
	echo "$l$s"
}
parent=$(dirname "$repo")
work=$parent/tt-wt
wd=$work/$run # the run's results; the run itself works in its slot's worktree (slot_wt)
unit=tt-${run//[^A-Za-z0-9_-]/_}
status=$work/$run.status
# The run slots: TT_MAXRUNS, or one per 6 cores and at least 2 (5 on a 32-core box) when it is unset.
slots=${TT_MAXRUNS:-}
case $slots in '' | *[!0-9]*) slots=$(($(nproc) / 6)); [ "$slots" -ge 2 ] || slots=2 ;; esac

drop() { # delete a run's leftovers
	systemctl --user stop "$unit.scope" >/dev/null 2>&1
	[ -z "$run" ] || { git -C "$repo" worktree remove --force "$wd" >/dev/null 2>&1; rm -rf -- "$wd" "$work/$run.patch" "$status"; }
	git -C "$repo" worktree prune >/dev/null 2>&1
	if [ -n "$run" ]; then
		git -C "$repo" update-ref -d "refs/tt/$run/head" >/dev/null 2>&1
		git -C "$repo" update-ref -d "refs/tt/$run/base" >/dev/null 2>&1
	fi
}

case $cmd in
probe)
	git -C "$repo" rev-parse --git-dir >/dev/null 2>&1 || { echo "probe: $repo is not a git repository on this box" >&2; exit 92; }
	mkdir -p "$work" || exit 92
	# Janitor: a client that died between the run and its cleanup leaves a worktree behind.
	find "$work" -mindepth 1 -maxdepth 1 -mmin +1440 ! -name '.slot*' -exec rm -rf {} + 2>/dev/null
	git -C "$repo" worktree prune >/dev/null 2>&1
	now=$(date +%s)
	git -C "$repo" for-each-ref --format='%(refname) %(committerdate:unix)' refs/tt 2>/dev/null | while read -r ref ts; do
		[ $((now - ${ts:-$now})) -lt 86400 ] || git -C "$repo" update-ref -d "$ref" >/dev/null 2>&1
	done
	# Idle cores: two /proc/stat samples a second apart.
	read -r _ u1 n1 s1 i1 w1 q1 sq1 st1 _ </proc/stat
	sleep 1
	read -r _ u2 n2 s2 i2 w2 q2 sq2 st2 _ </proc/stat
	idle=$(awk -v n="$(nproc)" -v t1="$((u1 + n1 + s1 + i1 + w1 + q1 + sq1 + st1))" -v t2="$((u2 + n2 + s2 + i2 + w2 + q2 + sq2 + st2))" \
		-v i1="$((i1 + w1))" -v i2="$((i2 + w2))" 'BEGIN { d = t2 - t1; v = 0; if (d > 0) v = n * (i2 - i1) / d; printf "%.1f", v }')
	free=0
	for i in $(seq 1 "$slots"); do
		{ exec 8>>"$work/.slot$i"; } 2>/dev/null || continue
		if flock -n 8; then free=$((free + 1)); fi
		exec 8>&-
	done
	lock=$(corpus_key "${TT_CARDS:-/nonexistent}")
	echo "probe idle=$idle free=$free slots=$slots lock=$lock"
	;;
cleanup)
	drop
	;;
finish)
	tar -C "$work" -cf - "$run.status" "$run/.ds4/scratch/test" 2>/dev/null
	drop
	;;
run)
	infra() { echo "infra=$*" >"$status"; echo "test-remote-box: $*" >&2; exit 92; }
	[ -d "$work" ] || infra "no $work"
	got=
	for i in $(seq 1 "$slots"); do
		{ exec 8>>"$work/.slot$i"; } 2>/dev/null || continue
		if flock -n 8; then got=$i; break; fi
		exec 8>&-
	done
	[ -n "$got" ] || { echo "busy=1" >"$status"; exit 91; }
	# One worktree per slot, reused, and only while this run holds the slot's flock (fd 8, open until this script
	# exits), so two runs never share it. go's test cache keys PWD and the absolute path and mtime of every module
	# file a test opens: a fresh worktree per run (the old tt-wt/<run>) could never hit, not even the same commit
	# twice. `checkout --force` rewrites only the files that differ, so an unchanged package keeps its mtimes and
	# its cached result (test-cache-audit 2026-11-14).
	slot_wt() {
		local w=$work/.slot$got.wt gd
		if [ -f "$w/.git" ] && gd=$(git -C "$w" rev-parse --absolute-git-dir 2>/dev/null); then
			# A run killed mid-checkout leaves the index lock; this slot's flock says nobody else holds it.
			rm -f "$gd/index.lock"
			if git -C "$w" checkout -q --force --detach "refs/tt/$run/head" && git -C "$w" clean -fdxq -e /.cards; then
				echo "$w"
				return 0
			fi
		fi
		# Missing, broken or half-made (a run killed inside `worktree add` leaves it registered and locked): make it anew.
		rm -rf -- "$w"
		git -C "$repo" worktree prune >/dev/null 2>&1
		git -C "$repo" worktree add -f -f -q --detach "$w" "refs/tt/$run/head" >/dev/null && echo "$w"
	}
	res=$wd
	wd=$(slot_wt) || infra "the slot worktree could not be set to the run's head"
	if [ -s "$work/$run.patch" ]; then
		git -C "$wd" apply --binary "$work/$run.patch" || infra "the uncommitted diff does not apply"
	fi
	[ -z "${TT_GOROOT:-}" ] || [ ! -d "$TT_GOROOT/bin" ] || export GOROOT=$TT_GOROOT PATH=$TT_GOROOT/bin:$PATH
	command -v go >/dev/null || infra "no go on this box"
	mkdir -p "$parent/gocache" "$parent/gopath" "$parent/tmp"
	export GOTOOLCHAIN=local GOFLAGS=-trimpath GOCACHE=$parent/gocache GOPATH=$parent/gopath GOWORK=off TMPDIR=$parent/tmp
	[ ! -d "$parent/pools" ] || export GORGE_POOLS=$parent/pools
	cards=${TT_CARDS:-}
	[ -n "$cards" ] && [ -d "$cards/cardsfolder" ] || infra "no corpus at '$cards'"
	ln -sfn "$cards" "$wd/.cards"
	# The worktree of a bare repository has no .git directory to find the shared corpus through.
	export GORGE_CARDS_DIR=$cards
	if [ -r /proc/sys/kernel/perf_event_paranoid ] && [ "$(cat /proc/sys/kernel/perf_event_paranoid)" -gt 2 ]; then
		sudo -n sysctl -q kernel.perf_event_paranoid=1 >/dev/null 2>&1 || true
	fi
	contended() { systemctl --user is-active rgate.scope ftrain.scope 2>/dev/null | grep -qx active; }
	c0=0
	! contended || c0=1
	cd "$wd" || infra "cd $wd"
	nl=${TT_NICE:-5}
	case $nl in '' | *[!0-9]*) nl=5 ;; esac
	inner=(nice -n "$nl" env BASE="refs/tt/$run/base" TEST_TARGETS_REMOTE=1 TEST_TARGETS_SCOPED=1 GOMAXPROCS="${TT_PROCS:-8}" GOMEMLIMIT="${TT_GOMEMLIMIT:-40GiB}" bash scripts/test-targets.sh "$mode")
	# One scope for the whole run: scopes the run opens inside would escape this CPUWeight and CPUQuota
	# (test-targets.sh opens none when TEST_TARGETS_REMOTE is set).
	if systemd-run --user --scope -q true >/dev/null 2>&1; then
		systemd-run --user --scope -q --unit="$unit" -p CPUWeight="${TT_WEIGHT:-20}" -p CPUQuota="${TT_CPU:-1200}%" -p MemoryMax="${TT_MEM:-48G}" "${inner[@]}"
	else
		echo "test-remote-box: note: no user systemd on the box; running uncapped" >&2
		"${inner[@]}"
	fi
	rc=$?
	# The results leave the slot before its lock does: the next run in this slot cleans the worktree.
	rm -rf -- "$res" && mkdir -p "$res/.ds4/scratch" && { [ ! -d "$wd/.ds4/scratch/test" ] || cp -R "$wd/.ds4/scratch/test" "$res/.ds4/scratch/"; }
	c1=0
	! contended || c1=1
	printf 'rc=%s\ncontended=%s\n' "$rc" "$((c0 + c1))" >"$status"
	exit 0
	;;
*)
	echo "usage: test-remote-box.sh probe|run|finish|cleanup <repo> [<run> <mode>]" >&2
	exit 2
	;;
esac
