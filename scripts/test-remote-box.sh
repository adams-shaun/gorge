#!/usr/bin/env bash
# test-remote-box.sh probe|run|finish|cleanup <bare-repo> [<run> <mode>]: the half of scripts/test-remote.sh that
# runs ON THE GATE BOX. test-remote.sh feeds it to `bash -s` over ssh, so the box needs no copy of it.
#
#   probe    <repo>              janitor (scratch worktrees and refs older than a day), then one line:
#                                probe idle=<cores> free=<run slots> lock=<corpus lock hash>
#   run      <repo> <run> <mode> a detached scratch worktree of refs/tt/<run>/head (+ <repo>/../tt-wt/<run>.patch),
#                                the same scripts/test-targets.sh <mode> in a systemd scope whose CPUWeight is below
#                                the landing gate's (rgate.scope runs at 100). It streams the output, writes
#                                <repo>/../tt-wt/<run>.status and exits 0; 91 = no free run slot; 92 = the box could
#                                not set the run up (the status file says why). Neither is a test result.
#   finish   <repo> <run>        tar (status + .ds4/scratch/test) to stdout, then cleanup
#   cleanup  <repo> <run>        stop the scope, remove the worktree, the refs, the patch and the status
#
# Settings arrive as TT_* environment variables (set by the client from ~/.config/gorge/test-targets.env):
# TT_MAXRUNS, TT_WEIGHT, TT_CPU (percent), TT_MEM, TT_PROCS, TT_CARDS (the corpus on this box), TT_GOROOT, TT_NICE
# (the run's nice level, default 5: below the landing gate's 0 as well as under its CPUWeight).
# Layout: <repo>/../tt-wt/<run> worktrees, <repo>/../gocache (the landing gate's build cache, kept warm),
# <repo>/../pools (deck pools, when present).
set -uo pipefail
cmd=${1:-}
repo=${2:-}
run=${3:-}
mode=${4:-}
parent=$(dirname "$repo")
work=$parent/tt-wt
wd=$work/$run
unit=tt-${run//[^A-Za-z0-9_-]/_}
status=$work/$run.status

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
	for i in $(seq 1 "${TT_MAXRUNS:-4}"); do
		{ exec 8>>"$work/.slot$i"; } 2>/dev/null || continue
		if flock -n 8; then free=$((free + 1)); fi
		exec 8>&-
	done
	lock=$(sha256sum "${TT_CARDS:-/nonexistent}/cards.lock" 2>/dev/null | cut -c1-16)
	echo "probe idle=$idle free=$free lock=$lock"
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
	for i in $(seq 1 "${TT_MAXRUNS:-4}"); do
		{ exec 8>>"$work/.slot$i"; } 2>/dev/null || continue
		if flock -n 8; then got=$i; break; fi
		exec 8>&-
	done
	[ -n "$got" ] || { echo "busy=1" >"$status"; exit 91; }
	git -C "$repo" worktree add -q --detach "$wd" "refs/tt/$run/head" || infra "git worktree add failed"
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
	inner=(nice -n "$nl" env BASE="refs/tt/$run/base" TEST_TARGETS_REMOTE=1 TEST_TARGETS_SCOPED=1 GOMAXPROCS="${TT_PROCS:-16}" GOMEMLIMIT="${TT_GOMEMLIMIT:-40GiB}" bash scripts/test-targets.sh "$mode")
	# One scope for the whole run: scopes the run opens inside would escape this CPUWeight and CPUQuota
	# (test-targets.sh opens none when TEST_TARGETS_REMOTE is set).
	if systemd-run --user --scope -q true >/dev/null 2>&1; then
		systemd-run --user --scope -q --unit="$unit" -p CPUWeight="${TT_WEIGHT:-20}" -p CPUQuota="${TT_CPU:-2400}%" -p MemoryMax="${TT_MEM:-48G}" "${inner[@]}"
	else
		echo "test-remote-box: note: no user systemd on the box; running uncapped" >&2
		"${inner[@]}"
	fi
	rc=$?
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
