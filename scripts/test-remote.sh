#!/usr/bin/env bash
# test-remote.sh: sourced by scripts/test-targets.sh. remote_try runs the mode on the gate box when it can.
# It either exits with the box's verdict (printed as the last line, exactly as a local run prints it) or returns
# so the caller runs the mode locally. An infrastructure failure of any kind returns: only a test failure on the
# box is FAIL. Ported from the landing gate's tools (rgate.sh, rgate-remote.sh, prescreen.sh): the git push into
# the box's bare repository, the detached scratch worktree, one systemd scope at a CPUWeight below the gate's,
# the build cache shared with the gate, red-while-the-gate-shared-the-box is not trusted, cleanup after every run.
#
# Settings (the machine's, in ~/.config/gorge/test-targets.env; unset means local only):
#   GORGE_REMOTE_HOST       user@host of the box (or GORGE_REMOTE_HOST_FILE: a "running <type> <az> <ip> ..." line)
#   GORGE_REMOTE_KEY        ssh identity file
#   GORGE_REMOTE_REPO       the bare repository on the box (its sibling tt-wt/ holds the scratch worktrees)
#   GORGE_REMOTE_MIN_IDLE   fewest idle cores on the box worth shipping for (default 24)
#   GORGE_REMOTE_OFF        1 turns the remote runner off
#   GORGE_REMOTE_MODES      the modes that go to the box (default "one targeted smart"); "targeted smart" keeps
#                           the light test-one here
#   GORGE_REMOTE_WAIT       seconds to wait for a free run slot on a busy box before running here (default 0: at
#                           once); GORGE_REMOTE_POLL is the re-probe interval (20)
#   GORGE_REMOTE_FALLBACK_CPU  the CPUQuota percent of a run that fell back here (default 200: -p 2, GOMAXPROCS 2),
#                           so a box outage does not put every seat's heavy run on this machine at the full cap
# Optional: GORGE_REMOTE_MAX_RUNS (concurrent remote runs, default 4), GORGE_REMOTE_WEIGHT (CPUWeight, default
# 20; the landing gate's rgate.scope runs at 100), GORGE_REMOTE_NICE (the run's nice level, default 5; the gate
# runs at 0), GORGE_REMOTE_CPU (CPUQuota percent, 2400), GORGE_REMOTE_MEM
# (48G), GORGE_REMOTE_PROCS (GOMAXPROCS, 16), GORGE_REMOTE_CARDS (the corpus path on the box, default this
# worktree's .cards target), GORGE_REMOTE_GOROOT (go on the box).
#
# Not shipped remotely (they run here): list, a smart run on a dirty tree, and any run that writes files
# (UPDATE, TESTENV). Output streams back; the box's own verdict line is held until the result is trusted.
# A BUDGET answer is trusted (the box has more cores than a run here); a red or a BUDGET while the landing gate
# shared the box is rerun there once, and the second answer is trusted.
# A run that falls back exports TEST_TARGETS_FALLBACK=1: test-targets.sh then caps it at GORGE_REMOTE_FALLBACK_CPU,
# and its capped re-exec does not try the box again.

rt_note() { echo "test-targets: note: $*" >&2; }

rt_ssh() { # rt_ssh COMMAND...: ssh to the box (-F /dev/null: the system config is unreadable in a sandbox)
	ssh -F /dev/null -i "$GORGE_REMOTE_KEY" -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new \
		-o ServerAliveInterval=20 -o ServerAliveCountMax=6 "$rt_host" "$@"
}

rt_box() { # rt_box VERB RUN [MODE]: a verb of test-remote-box.sh, with the settings the box script reads
	local a=(env "TT_MAXRUNS=$rt_maxruns" "TT_WEIGHT=${GORGE_REMOTE_WEIGHT:-20}" "TT_CPU=${GORGE_REMOTE_CPU:-2400}"
		"TT_MEM=${GORGE_REMOTE_MEM:-48G}" "TT_PROCS=${GORGE_REMOTE_PROCS:-16}" "TT_CARDS=$rt_cards"
		"TT_NICE=${GORGE_REMOTE_NICE:-5}")
	[ -z "${GORGE_REMOTE_GOROOT:-}" ] || a+=("TT_GOROOT=$GORGE_REMOTE_GOROOT")
	local v
	if [ "$1" = run ]; then
		for v in PKG RUN SKIP V TIMEOUT TAGS; do [ -z "${!v:-}" ] || a+=("$v=${!v}"); done
	fi
	a+=(bash -s -- "$1" "$GORGE_REMOTE_REPO" "${@:2}")
	timeout "${RT_TIMEOUT:-7200}" ssh -F /dev/null -i "$GORGE_REMOTE_KEY" -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new \
		-o ServerAliveInterval=20 -o ServerAliveCountMax=6 "$rt_host" "$(printf '%q ' "${a[@]}")" <"$rt_script"
}

rt_filter() { # rt_filter VERDICT_FILE LOG: stream the box's output, holding its verdict line back
	local l
	while IFS= read -r l; do
		case $l in
		test-one:\ * | test-targeted:\ * | test-smart:\ *) printf '%s\n' "$l" >"$1" ;;
		*) printf '%s\n' "$l"; printf '%s\n' "$l" >>"$2" ;;
		esac
	done
}

rt_cleanup() { [ -z "${rt_run:-}" ] || rt_box cleanup "$rt_run" >/dev/null 2>&1; }

rt_fallback() { # rt_fallback MSG: run here, at the fallback cap, and never try the box again in this run
	rt_note "$1"
	export TEST_TARGETS_FALLBACK=1
	return 0
}

remote_try() {
	[ -z "${TEST_TARGETS_REMOTE:-}" ] || return 0 # this IS the remote run
	[ -z "${TEST_TARGETS_FALLBACK:-}" ] || return 0 # the capped re-exec of a run that already fell back
	[ "${GORGE_REMOTE_OFF:-}" != 1 ] || return 0
	[ "$mode" != list ] || return 0
	case " ${GORGE_REMOTE_MODES:-one targeted smart} " in *" $mode "*) ;; *) return 0 ;; esac
	[ -n "${GORGE_REMOTE_HOST:-}${GORGE_REMOTE_HOST_FILE:-}" ] && [ -n "${GORGE_REMOTE_KEY:-}" ] && [ -n "${GORGE_REMOTE_REPO:-}" ] || return 0 # not configured
	# Runs that write files into the worktree, and a smart run that will refuse a dirty tree, stay here.
	[ -z "${UPDATE:-}${TESTENV:-}" ] || return 0
	if [ "$mode" = smart ] && [ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]; then return 0; fi
	rt_host=${GORGE_REMOTE_HOST:-}
	if [ -z "$rt_host" ]; then
		local st ip
		read -r st _ _ ip _ <"$GORGE_REMOTE_HOST_FILE" 2>/dev/null
		if [ "$st" = running ] && [ -n "$ip" ]; then
			rt_host=${GORGE_REMOTE_USER:-ubuntu}@$ip
		else
			rt_fallback "remote: $GORGE_REMOTE_HOST_FILE says the box is '${st:-missing}'; running locally"
			return 0
		fi
	fi
	[ -r "$GORGE_REMOTE_KEY" ] || { rt_fallback "remote: key $GORGE_REMOTE_KEY is not readable; running locally"; return 0; }
	rt_script=$(dirname "${BASH_SOURCE[0]}")/test-remote-box.sh
	[ -r "$rt_script" ] || { rt_fallback "remote: no $rt_script; running locally"; return 0; }
	rt_maxruns=${GORGE_REMOTE_MAX_RUNS:-4}
	rt_cards=${GORGE_REMOTE_CARDS:-$(readlink -f .cards 2>/dev/null)}
	local wait=${GORGE_REMOTE_WAIT:-0} poll=${GORGE_REMOTE_POLL:-20} waited=0
	rt_reruns_left=1
	case $wait in '' | *[!0-9]*) wait=0 ;; esac
	case $poll in '' | *[!0-9]* | 0) poll=20 ;; esac
	while :; do
		rt_run=
		rt_why=
		rt_code=
		if rt_try_run; then
			echo "$rt_vline"
			exit "$rt_code"
		fi
		rt_cleanup
		case $rt_why in
		busy:*)
			# A busy box is still the runner: wait for a slot (the box's weights keep the landing gate first).
			[ "$waited" -lt "$wait" ] || break
			rt_note "remote $rt_host: $rt_why; waiting for a slot (${waited}s of ${wait}s)"
			sleep "$poll"
			waited=$((waited + poll))
			;;
		contended:*)
			# Red while the landing gate shared the box: once more there, then that answer stands.
			rt_reruns_left=0
			rt_note "remote $rt_host: ${rt_why#contended: }; running it on the box again"
			;;
		*) break ;;
		esac
	done
	rt_fallback "remote $rt_host: $rt_why; running locally"
	rt_run=
	return 0
}

# rt_try_run: 0 when the box's verdict is trusted (rt_vline, rt_code set, the json copied back), 1 to fall back
# (rt_why says why). Everything in between is infrastructure and returns 1.
rt_try_run() {
	local base_sha head_sha out idle free lock llock ti patch= vf lg x rrc sf vst contended
	local minidle=${GORGE_REMOTE_MIN_IDLE:-24}
	head_sha=$(git rev-parse HEAD 2>/dev/null) || { rt_why="no HEAD"; return 1; }
	base_sha=$(git rev-parse --verify -q "$BASE^{commit}") || { rt_why="BASE does not resolve"; return 1; }
	llock=$(sha256sum .cards/cards.lock 2>/dev/null | cut -c1-16)
	[ -n "$llock" ] || { rt_why="no local .cards/cards.lock to compare with the box's"; return 1; }
	out=$(RT_TIMEOUT=30 rt_box probe "" 2>/dev/null) || { rt_why="unreachable or not set up (ssh)"; return 1; }
	out=$(grep -m1 '^probe ' <<<"$out")
	idle=$(sed -n 's/.* idle=\([0-9.]*\).*/\1/p' <<<"$out")
	free=$(sed -n 's/.* free=\([0-9]*\).*/\1/p' <<<"$out")
	lock=$(sed -n 's/.* lock=\([0-9a-f]*\).*/\1/p' <<<"$out")
	[ -n "$idle" ] && [ -n "$free" ] || { rt_why="unreadable probe '$out'"; return 1; }
	[ "$free" -gt 0 ] || { rt_why="busy: all $rt_maxruns remote run slots are taken"; return 1; }
	awk -v i="$idle" -v m="$minidle" 'BEGIN { exit !(i >= m) }' || { rt_why="busy: $idle idle cores, need $minidle"; return 1; }
	[ "$lock" = "$llock" ] || { rt_why="the box's corpus differs from this one (cards.lock)"; return 1; }
	rt_run=$(id -un)-$$-$(date +%s)
	# The uncommitted diff, untracked files included, as a patch (a private index: this worktree's is untouched).
	if [ "$mode" != smart ]; then
		ti=$(mktemp "${TMPDIR:-/tmp}/tt-index.XXXXXX") && patch=$(mktemp "${TMPDIR:-/tmp}/tt-patch.XXXXXX") || { rt_why="no tmp dir"; return 1; }
		cp "$(git rev-parse --git-path index)" "$ti" 2>/dev/null
		if ! { GIT_INDEX_FILE=$ti git add -A -- . && GIT_INDEX_FILE=$ti git diff --cached --binary HEAD >"$patch"; } 2>/dev/null; then
			rm -f "$ti" "$patch"; rt_why="could not make the patch"; return 1
		fi
		rm -f "$ti"
		if [ "$(wc -c <"$patch")" -gt 33554432 ]; then rm -f "$patch"; rt_why="the uncommitted diff is over 32 MB"; return 1; fi
	fi
	if ! GIT_SSH_COMMAND="ssh -F /dev/null -i $GORGE_REMOTE_KEY -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new" \
		git push -q -f "$rt_host:$GORGE_REMOTE_REPO" "$head_sha:refs/tt/$rt_run/head" "$base_sha:refs/tt/$rt_run/base" </dev/null >/dev/null 2>&1; then
		rm -f "$patch"; rt_why="git push failed"; return 1
	fi
	if [ -s "$patch" ]; then
		if ! timeout 120 ssh -F /dev/null -i "$GORGE_REMOTE_KEY" -o BatchMode=yes -o ConnectTimeout=5 "$rt_host" \
			"$(printf 'mkdir -p %q && cat > %q' "$(dirname "$GORGE_REMOTE_REPO")/tt-wt" "$(dirname "$GORGE_REMOTE_REPO")/tt-wt/$rt_run.patch")" <"$patch" 2>/dev/null; then
			rm -f "$patch"; rt_why="could not send the uncommitted diff"; return 1
		fi
	fi
	[ -z "$patch" ] || rm -f "$patch"
	echo "== remote: $rt_host, $idle idle cores, run $rt_run"
	trap 'rt_cleanup; exit 130' INT TERM
	vf=$(mktemp "${TMPDIR:-/tmp}/tt-verdict.XXXXXX") lg=$(mktemp "${TMPDIR:-/tmp}/tt-log.XXXXXX") x=$(mktemp -d "${TMPDIR:-/tmp}/tt-back.XXXXXX")
	rt_box run "$rt_run" "$mode" | rt_filter "$vf" "$lg"
	rrc=${PIPESTATUS[0]}
	rt_box finish "$rt_run" 2>/dev/null | tar -C "$x" -xf - 2>/dev/null
	trap - INT TERM
	sf=$x/$rt_run.status
	if [ "$rrc" = 91 ] || grep -q '^busy=1' "$sf" 2>/dev/null; then rt_why="busy: no free run slot on the box"
	elif grep -q '^infra=' "$sf" 2>/dev/null; then rt_why="box setup failed: $(sed -n 's/^infra=//p' "$sf")"
	elif [ "$rrc" != 0 ] || ! grep -q '^rc=' "$sf" 2>/dev/null; then rt_why="the run did not complete (ssh rc=$rrc)"
	fi
	if [ -z "$rt_why" ]; then
		rt_vline=$(cat "$vf" 2>/dev/null)
		vst=$(sed -n "s/^test-$mode: \([A-Z]*\) .*/\1/p" <<<"$rt_vline")
		contended=$(sed -n 's/^contended=//p' "$sf")
		case $vst in
		PASS) rt_code=0 ;;
		EMPTY) rt_code=4 ;;
		FAIL | BUDGET)
			rt_code=1
			[ "$vst" = FAIL ] || rt_code=5
			# Neither a box that shared its cores with the landing gate nor the dirac gate's own infrastructure
			# exit (rc 2) says anything about this change; rgate.sh trusts neither. A BUDGET on the box (more
			# cores than here) would be a BUDGET here too, so it stands unless the gate shared the box.
			if [ "${contended:-0}" != 0 ] && [ "${rt_reruns_left:-1}" != 0 ]; then rt_why="contended: ${vst} on the box while the landing gate shared it"
			elif [ "$vst" = FAIL ] && grep -q 'dirac gate rc=2' "$lg"; then rt_why="the dirac gate reported an infrastructure failure"; fi
			;;
		*) rt_why="the box answered ${vst:-nothing} (${rt_vline:-no verdict line})" ;;
		esac
	fi
	if [ -z "$rt_why" ]; then
		# Trusted: the full go test -json (and smart's logs) go to the usual local path.
		mkdir -p .ds4/scratch/test
		[ ! -d "$x/$rt_run/.ds4/scratch/test" ] || cp -R "$x/$rt_run/.ds4/scratch/test/." .ds4/scratch/test/ 2>/dev/null
	fi
	rm -rf "$vf" "$lg" "$x"
	[ -z "$rt_why" ]
}
