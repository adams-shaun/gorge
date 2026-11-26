#!/usr/bin/env bash
# test-targets.sh <one|targeted|smart|list>: the make targets that replace a raw `go test`.
#
#   make test-one PKG=./p RUN='^TestX$'   one package, named tests: debugging, regeneration, scratch probes
#   [SHORT=1] adds -short: slow self-play/search tests skip (the Mergify batch's full suite still runs them)
#   make test-targeted [BASE=origin/main]        packages the diff touches, plus every package whose tests import them
#   make test-smart [BASE=origin/main]           the steps a seat can act on, fail-fast, on a committed tree
#   make precheck                         an alias for test-smart
#
# test-smart's step 4 is GORGE_TEST_SMART_GATES's:
#   unset   narrow (the default): the affected packages plus the test groups -- test-targeted's selection with
#           ./engine/rules whole when the change reaches it, the oracle's fast tier instead of its lockstep, the
#           lint packages, and the census step widened by the pin/layout/lint/replay names (coverage audit
#           2026-11-14, section 5). The prescreen and the lander run the affected and dirac gates.
#   1       gates: the affected-package gate and the dirac gate side by side, as the lander runs them.
#   0       off: neither; only the repro tables when the diff touches them.
#
# Each run sits in one user systemd scope (MemoryMax, CPUQuota, GOMAXPROCS, GOMEMLIMIT) and exports
# GORGE_TEST_WRAPPER=1. With no user systemd it says so and runs uncapped.
# Preflight: the .cards corpus resolves and is not empty, dirac's tables exist when a dirac package runs,
# BASE resolves. Postflight (scripts/test_targets.py): a skip that names the corpus, pools or tables is ENV,
# a -run that matched nothing is EMPTY, a timeout or kill is BUDGET (BUSY in a run that fell back from the gate box
# to a cap below 4 cores: the budget is set for 4).
#
# The last line on stdout is the one verdict line, and nothing else printed starts with "test-":
#
#   test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>
#
# Settings that belong to the machine, not the repository, go in ~/.config/gorge/test-targets.env (or
# $GORGE_TEST_TARGETS_ENV): GORGE_POOLS (dirac's deck pools), GATE_AFFECTED_REPO (+ GATE_AFFECTED_PATH, GATE_REF),
# GORGE_HEAVY_LOCK (+ GORGE_HEAVY_LANES, GORGE_TEST_LANE_WAIT), and the remote runner's GORGE_REMOTE_HOST (or
# GORGE_REMOTE_HOSTS, several boxes), GORGE_REMOTE_KEY, GORGE_REMOTE_REPO, GORGE_REMOTE_MIN_IDLE, GORGE_REMOTE_OFF (scripts/test-remote.sh).
#
# Runner: with the GORGE_REMOTE_* settings present, the mode runs on the gate box instead (the worktree's HEAD and
# its uncommitted diff are shipped; the same verdict line and the same go test -json at the same local path come
# back). An unreachable box, a busy one after GORGE_REMOTE_WAIT, or any infrastructure failure runs it here, with
# one note and at the lower GORGE_REMOTE_FALLBACK_CPU cap: only a test failure (or a BUDGET) on the box is the answer.
# GORGE_REMOTE_MODES picks the modes that go (e.g. "targeted smart", keeping test-one here). Unset (the default)
# means local only.
#
# exit 0 PASS, 1 FAIL, 2 usage, 3 ENV, 4 EMPTY, 5 BUDGET, 75 BUSY. The full `go test -json` stream is kept
# at .ds4/scratch/test/<mode>-<sha>.json. Never passes -count (the cache stays on).
set -uo pipefail
mode=${1:-}
case $mode in one | targeted | smart | list) ;; *) echo "usage: test-targets.sh one|targeted|smart|list" >&2; exit 2 ;; esac
top=$(git rev-parse --show-toplevel) || exit 2
cd "$top" || exit 2
# The operator's settings (GORGE_POOLS, GATE_AFFECTED_REPO, GORGE_HEAVY_LOCK, ...), outside the repository.
envfile=${GORGE_TEST_TARGETS_ENV:-${XDG_CONFIG_HOME:-$HOME/.config}/gorge/test-targets.env}
# shellcheck disable=SC1090
[ ! -r "$envfile" ] || { set -a; . "$envfile"; set +a; }
py=(python3 -I "$top/scripts/test_targets.py")
# timeout_for PKG: the one -timeout every mode uses for that package. The table lives in test_targets.py so the
# shell, the Python selector and the dirac gate cannot drift into a second cache entry for the same package.
timeout_for() { "${py[@]}" timeout --pkg "$1"; }
# group_by_timeout PKG...: set G_TO and G_PKGS to the packages grouped by their timeout, in first-seen order.
# A `go test` call takes one -timeout, so packages with a different one run in a separate call.
group_by_timeout() {
	G_TO=() G_PKGS=()
	local p to i
	for p in "$@"; do
		to=$(timeout_for "$p") || return 1
		for i in "${!G_TO[@]}"; do
			[ "${G_TO[i]}" = "$to" ] || continue
			G_PKGS[i]+=" $p"
			continue 2
		done
		G_TO+=("$to")
		G_PKGS+=("$p")
	done
}
BASE=${BASE:-origin/main}
t0=$SECONDS
sha=$(git rev-parse --short=10 HEAD 2>/dev/null || echo -)
[ -z "$(git status --porcelain --untracked-files=no 2>/dev/null)" ] || sha=$sha+dirty
pkgs_label=""
# shellcheck disable=SC1091
. "$top/scripts/test-remote.sh"

# record_verdict STATUS LINE: remember a terminal verdict (PASS or FAIL) for the commit, next to the logs, so
# a rerun on the same commit with the same gate scripts reuses it instead of repeating the work. ENV, EMPTY,
# BUDGET and BUSY describe the box, not the commit, and are never recorded.
# A FAIL also keeps its detail (the failing tests and their messages) in .verdict.detail: a reused FAIL that printed
# only its verdict line left the seat nothing to fix, and it reported DONE twice on a passing local run and parked.
record_verdict() {
	local v=.ds4/scratch/test/smart-${sha%+dirty}.verdict
	case $1 in
	PASS) printf '%s\n' "$2" >"$v"; rm -f "$v.detail" ;;
	FAIL) printf '%s\n' "$2" >"$v"; if [ -n "${3:-}" ]; then printf '%s\n' "$3" >"$v.detail"; else rm -f "$v.detail"; fi ;;
	esac
}

# verdict STATUS [msg]: print the verdict line and leave with its exit code.
verdict() {
	local st=$1 msg=${2:-} out rc
	out=$("${py[@]}" verdict --mode "$mode" --status "$st" --sha "$sha" --pkgs "$pkgs_label" --seconds $((SECONDS - t0)) ${msg:+--msg "$msg"})
	rc=$?
	printf '%s\n' "$out"
	[ "$mode" = smart ] && record_verdict "$st" "$out"
	exit $rc
}
usage() { echo "test-targets: $mode: $1" >&2; exit 2; }

# ---- the runner: the gate box when it can, else here --------------------------------------------------
remote_try
# The local cap of the parallel steps (test-targeted's -p, test-smart's capped steps): 4 cores, or the fallback
# cap when the configured box could not take the run.
lp=4 lcpu=400% lprocs=4
fcpu=${GORGE_REMOTE_FALLBACK_CPU:-200}
case $fcpu in '' | *[!0-9]* | 0 | ?) fcpu=200 ;; esac
fprocs=$((fcpu / 100))
[ -z "${TEST_TARGETS_FALLBACK:-}" ] || [ -n "${TEST_TARGETS_REMOTE:-}" ] || { lp=$fprocs lcpu=$fcpu% lprocs=$fprocs; }
# A fallback run below the budget's 4 cores: a timeout there is the machine's (BUSY), not the change's BUDGET. The
# 11-14 seat 3c80ac4f was parked for a dirac/oracle named run that timed out at 5m on one core (CPUQuota 100%).
fbcap=
[ -z "${TEST_TARGETS_FALLBACK:-}" ] || [ -n "${TEST_TARGETS_REMOTE:-}" ] || [ "$fprocs" -ge 4 ] || fbcap=$fcpu%

# ---- the cap -----------------------------------------------------------------------------------------
if [ -z "${TEST_TARGETS_SCOPED:-}" ]; then
	export GORGE_TEST_WRAPPER=1
	mem=4G lim=3GiB cpu=400% procs=4
	if [ "$mode" = targeted ] || [ "${PKG:-}" = ./dirac/oracle ]; then mem=8G lim=6GiB; fi
	# A run that fell back from the configured gate box: the box is down, so every seat's heavy run lands here at
	# once; cap it lower (test-remote.sh, GORGE_REMOTE_FALLBACK_CPU).
	[ -z "${TEST_TARGETS_FALLBACK:-}" ] || { cpu=$fcpu%; procs=$fprocs; }
	# A host-wide run slot (11-23): every seat's local run was capped at 4 cores, but nothing capped how many ran at
	# once. 22 ran together on the 32-core home box (load 99), and tests that pass alone hit the 60 s budget. A run
	# takes one of GORGE_LOCAL_SLOTS flock slots (default nproc/4; 0 turns it off) and holds it through the exec below,
	# so it is released when the run exits. It waits up to GORGE_LOCAL_SLOT_WAIT seconds (default 1800), then BUSY.
	if [ "$mode" != list ] && [ "$mode" != smart ]; then
		nslots=${GORGE_LOCAL_SLOTS:-$(($(nproc) / 4))}
		case $nslots in '' | *[!0-9]*) nslots=1 ;; esac
		if [ "$nslots" -gt 0 ]; then
			sdir=${GORGE_LOCAL_SLOT_DIR:-$HOME/.cache/gorge/local-test-slots}
			swait=${GORGE_LOCAL_SLOT_WAIT:-1800}
			case $swait in '' | *[!0-9]*) swait=1800 ;; esac
			if mkdir -p "$sdir" 2>/dev/null; then
				sdeadline=$((SECONDS + swait)) slot= snoted=
				while :; do
					for i in $(seq 1 "$nslots"); do
						exec 8>>"$sdir/slot$i" || continue
						if flock -n 8; then slot=$i; break; fi
						exec 8>&-
					done
					[ -z "$slot" ] || break
					[ "$SECONDS" -lt "$sdeadline" ] || verdict BUSY "all $nslots local test slots ($sdir) stayed busy for ${swait}s; finish with a BUSY concern instead of retrying"
					[ -n "$snoted" ] || { echo "test-targets: note: all $nslots local test slots are busy; waiting for one" >&2; snoted=1; }
					sleep 2
				done
			else
				echo "test-targets: note: cannot create $sdir; running without a local slot" >&2
			fi
		fi
	fi
	if [ "$mode" = list ] || [ "$mode" = smart ]; then
		: # list runs go list only; smart caps each of its own steps
	elif systemd-run --user --scope -q true >/dev/null 2>&1; then
		exec systemd-run --user --scope -q -p MemoryMax=$mem -p CPUQuota=$cpu env TEST_TARGETS_SCOPED=1 GOMAXPROCS=$procs GOMEMLIMIT=$lim bash "$top/scripts/test-targets.sh" "$@"
	else
		echo "test-targets: note: no user systemd; running uncapped" >&2
	fi
fi
export GORGE_TEST_WRAPPER=1 GOFLAGS=-trimpath
GOROOT=$(go env GOROOT) && export GOROOT

if [ "$mode" = list ]; then exec "${py[@]}" select --base "$BASE"; fi

# ---- preflight ---------------------------------------------------------------------------------------
cards=${GORGE_CARDS:-.cards}
[ -n "$(ls -A "$cards/cardsfolder" 2>/dev/null | head -1)" ] ||
	verdict ENV "no corpus: $cards/cardsfolder is missing or empty, so every corpus test would skip. run scripts/link-cards.sh: it links the corpus of this tree's own FORGE_REF"
git rev-parse --verify -q "$BASE^{commit}" >/dev/null || verdict ENV "base $BASE does not resolve in this worktree"
git merge-base --is-ancestor "$BASE" HEAD || echo "test-targets: note: HEAD is not on top of $BASE; the landing rebases, so a conflict with it is not covered" >&2
mod=$(go list -m) || verdict ENV "go list -m failed"
mkdir -p .ds4/scratch/test
json=.ds4/scratch/test/$mode-${sha%+dirty}.json
: >"$json"

# capped MEM CPU cmd...: the command in its own user scope; smart has no outer one. Uncapped without user systemd.
have_sd=
# On the box the whole run is already one scope (CPUWeight below the landing gate's): a scope opened inside it
# would escape that weight, so nothing here opens one.
[ -n "${TEST_TARGETS_REMOTE:-}" ] || ! systemd-run --user --scope -q true >/dev/null 2>&1 || have_sd=1
capped() {
	if [ -n "$have_sd" ]; then systemd-run --user --scope -q -p MemoryMax="$1" -p CPUQuota="$2" "${@:3}"; else "${@:3}"; fi
}
dirac_tables() { # needed before any dirac package plays a game; a no-op when the tables are current
	local c=(); [ -n "${TEST_TARGETS_SCOPED:-}" ] || c=(capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB)
	"${c[@]}" make -s dirac-tables >.ds4/scratch/test/dirac-tables.log 2>&1 || verdict ENV "make dirac-tables failed: .ds4/scratch/test/dirac-tables.log"
}

# gojson TIMEOUT args...: one `go test -json` call appended to $json; sets rc to the worst status so far.
# -json, -v and -failfast are always the same flags: they are part of go's test-cache key.
rc=0
gojson() {
	local to=$1
	shift
	go test -json -v -failfast -timeout "$to" "$@" >>"$json" 2>&1
	local r=$?
	[ "$r" -le "$rc" ] || rc=$r
	return $r
}
finish() { # finish NAMED_IMPORT_PATHS: classify $json, print detail and the verdict, exit
	local out pyrc vline st
	out=$("${py[@]}" finish --mode "$mode" --json "$json" --rc "$rc" --sha "$sha" --pkgs "$pkgs_label" --seconds $((SECONDS - t0)) --named "$1" --base "$BASE" ${fbcap:+--fallback "$fbcap"})
	pyrc=$?
	printf '%s\n' "$out"
	if [ "$mode" = smart ]; then
		vline=${out##*$'\n'}
		st=${vline#test-smart: }
		st=${st%% *}
		detail=""; [ "$out" = "$vline" ] || detail=${out%$'\n'*}   # all but the verdict line itself
		record_verdict "$st" "$vline" "$detail"
	fi
	exit $pyrc
}

# ---- test-one ----------------------------------------------------------------------------------------
if [ "$mode" = one ]; then
	pkg=${PKG:-}
	[[ $pkg =~ ^\./[A-Za-z0-9_./-]+$ && $pkg != *...* ]] || usage "PKG=./path/to/one/package is required (got '$pkg'); no ./... and no wildcards"
	[ -n "${RUN:-}" ] || usage "RUN=<test regexp> is required"
	case ${SHORT:-} in '' | 1) ;; *) usage "SHORT is 1 or empty (got '$SHORT')" ;; esac
	case $pkg in ./engine/rules | ./dirac/oracle)
		# An exact name (or a |-alternation of them), so a broad pattern cannot run the whole package; it may end in
		# one /subtest pattern ('^TestPinManifest$/^TestX\.0$': re-record one pin without the whole manifest's budget).
		[[ $RUN =~ ^\^[A-Za-z0-9_|^$]*[A-Za-z0-9_]\$(/[][A-Za-z0-9_.|^$\\()-]+)?$ ]] ||
			usage "$pkg is too big to run whole: name the exact tests in RUN (e.g. '^TestX\$'); got '$RUN'"
		;;
	esac
	# The package's table timeout, the same one targeted and smart pass to this package's runs: -timeout is part
	# of go's test-cache key, so a test-one at 2m and the same -run at 5m were two cache entries and every test-one
	# result re-ran in targeted (test-cache-audit 2026-11-14). TIMEOUT is an explicit debugging override.
	to=${TIMEOUT:-$(timeout_for "$pkg")}
	[[ $to =~ ^([0-9]+)([sm])$ ]] || usage "TIMEOUT is Ns or Nm"
	secs=${BASH_REMATCH[1]}
	[ "${BASH_REMATCH[2]}" = s ] || secs=$((secs * 60))
	[ "$secs" -ge 1 ] && [ "$secs" -le 600 ] || usage "TIMEOUT is between 1s and 10m"
	[ -z "${TAGS:-}" ] || [[ $TAGS =~ ^[A-Za-z0-9_,]+$ ]] || usage "TAGS is a comma list of build tags"
	envs=()
	for kv in ${TESTENV:-}; do
		case ${kv%%=*} in KGEN_REPIN | DIRAC_PIN_STRICT | DIRAC_SUPPORT_STRICT | DIRAC_UPDATE_SUPPORT | DIRAC_UPDATE_LAYOUT | KOP_ORDINALS_REGEN | PINS_RECORD | CMD5_CLEAN_REPIN | MANAFLARE_CAST_RECORD | CRYOSHATTER_RECORD | MAGESLAYER_RECORD | GORGE_POOLS) envs+=("$kv") ;;
		*) usage "TESTENV allows only KGEN_REPIN DIRAC_PIN_STRICT DIRAC_SUPPORT_STRICT DIRAC_UPDATE_SUPPORT DIRAC_UPDATE_LAYOUT KOP_ORDINALS_REGEN PINS_RECORD CMD5_CLEAN_REPIN MANAFLARE_CAST_RECORD CRYOSHATTER_RECORD MAGESLAYER_RECORD GORGE_POOLS (got ${kv%%=*})" ;;
		esac
	done
	pkgs_label=$pkg
	case $pkg in ./dirac*) dirac_tables ;; esac
	args=(-run "$RUN")
	[ -z "${SKIP:-}" ] || args+=(-skip "$SKIP")
	[ -z "${TAGS:-}" ] || args+=(-tags "$TAGS")
	case ${SHORT:-} in 1) args+=(-short) ;; esac
	args+=("$pkg")
	[ -z "${UPDATE:-}" ] || args+=(-args -update)
	env ${envs[@]+"${envs[@]}"} go test -json -v -failfast -timeout "$to" "${args[@]}" >>"$json" 2>&1
	rc=$?
	finish "$mod/${pkg#./}"
fi

# ---- test-targeted -----------------------------------------------------------------------------------
if [ "$mode" = targeted ]; then
	sel=$("${py[@]}" select --base "$BASE") || verdict FAIL "selection failed: $sel"
	whole=() named=() runs=() dirac=0
	while IFS=$'\t' read -r kind pkg rx; do
		[ -n "$kind" ] || continue
		case $pkg in ./dirac*) dirac=1 ;; esac
		case $kind in
		whole) whole+=("$pkg") ;;
		named) runs+=("$pkg"$'\t'"$rx"); named+=("$mod/${pkg#./}") ;;
		census) runs+=("$pkg"$'\t'"$rx") ;;
		excluded) echo "test-targets: note: $pkg imports a changed package but is over budget; it runs post-merge" >&2; continue ;;
		esac
		pkgs_label=${pkgs_label:+$pkgs_label,}$pkg
	done <<<"$sel"
	echo "== selection: ${#whole[@]} whole, ${#runs[@]} named/census"
	for w in "${whole[@]+"${whole[@]}"}"; do echo "   whole  $w"; done
	for r in "${runs[@]+"${runs[@]}"}"; do echo "   named  ${r%%$'\t'*}"; done
	# gofmt and vet on the Go files this change touches.
	mb=$(git merge-base "$BASE" HEAD)
	files=$({ git diff --name-only --diff-filter=d "$mb" -- '*.go'; git ls-files -o --exclude-standard -- '*.go'; } | sort -u)
	if [ -n "$files" ]; then
		unfmt=$(gofmt -l $files)
		[ -z "$unfmt" ] || { echo "gofmt: $unfmt"; verdict FAIL "gofmt"; }
		vp=$(for f in $files; do echo "./$(dirname "$f")"; done | sort -u | sed 's|^\./\.$|.|')
		vo=$(go vet $vp 2>&1) || { tail -15 <<<"$vo"; verdict FAIL "go vet"; }
	fi
	[ "$dirac" = 0 ] || dirac_tables
	# The cheap named/census runs first: a whole-package BUDGET below must not hide their failures.
	for r in "${runs[@]+"${runs[@]}"}"; do
		gojson "$(timeout_for "${r%%$'\t'*}")" -run "${r#*$'\t'}" "${r%%$'\t'*}" || finish ""
	done
	if [ "${#whole[@]}" -gt 0 ]; then
		group_by_timeout "${whole[@]}" || finish ""
		for i in "${!G_TO[@]}"; do
			read -r -a group <<<"${G_PKGS[i]}"
			gojson "${G_TO[i]}" -p "$lp" "${group[@]}" || finish ""
		done
	fi
	finish "$(IFS=,; echo "${named[*]-}")"
fi

# ---- test-smart --------------------------------------------------------------------------------------
# The steps a seat can act on, fail-fast (fleet/land/tools/wland2.sh and g-land.sh): gofmt, the docs
# snapshot, history hygiene, the oracle skip lint; the census tests; step 4; then the wire goldens. Step 4 is
# GORGE_TEST_SMART_GATES's (the header): by default the narrow affected-plus-groups set; =1 the
# affected-package gate (GATE_AFFECTED_REPO's GATE_REF) and the dirac gate (BASE's scripts/dirac-gate.sh) side
# by side in a heavy lane, the repro tables also running when the dirac gate skipped; =0 only the repro
# tables when the diff touches dirac/oracle/testdata/repro/. precheck.sh's census, unrefusal and 3b repro
# steps stay in, so the alias loses nothing. Not run here (follow-ups in the report): wland2's pinrise
# re-pin and g-land's remote/farm (rgate) path.
[ -z "$(git status --porcelain --untracked-files=no)" ] || verdict ENV "commit first: test-smart checks a commit, not a dirty tree"
mb=$(git merge-base "$BASE" HEAD)
bad() { echo "test-targets: smart: $*" >&2; verdict FAIL "$*"; }
case ${GORGE_TEST_SMART_GATES:-} in 1) smart=gates ;; 0) smart=off ;; *) smart=narrow ;; esac
if [ "$smart" = gates ]; then
	# The affected-package gate lives in another repository; the operator's env file names it. With no
	# setting, look for a local checkout one level under $HOME that carries the gate at GATE_REF, so
	# `make precheck` still runs on a box whose environment file is not provisioned.
	GATE_AFFECTED_PATH=${GATE_AFFECTED_PATH:-ops/scripts/gate_affected.sh}
	GATE_REF=${GATE_REF:-main}
	if [ -z "${GATE_AFFECTED_REPO:-}" ] && [ -n "${HOME:-}" ]; then
		for d in "$HOME"/*/*; do
			[ -e "$d/.git" ] || continue
			if git -C "$d" cat-file -e "$GATE_REF:$GATE_AFFECTED_PATH" 2>/dev/null; then GATE_AFFECTED_REPO=$d; break; fi
		done
	fi
	[ -n "${GATE_AFFECTED_REPO:-}" ] || verdict ENV "GATE_AFFECTED_REPO is not set and no local checkout under \$HOME carries $GATE_AFFECTED_PATH at $GATE_REF (set it in $envfile)"
	aff_sha=$(git -C "$GATE_AFFECTED_REPO" rev-parse --short=7 "$GATE_REF:$GATE_AFFECTED_PATH" 2>/dev/null) ||
		verdict ENV "$GATE_AFFECTED_PATH not found in $GATE_AFFECTED_REPO at $GATE_REF: set GATE_AFFECTED_REPO, GATE_AFFECTED_PATH, GATE_REF"
	dg_sha=$(git rev-parse --short=7 "$BASE:scripts/dirac-gate.sh" 2>/dev/null) || verdict ENV "$BASE has no scripts/dirac-gate.sh"
	# The step-4 lane wait, clamped to 1500s whatever the environment says: a seat that waits longer than the
	# gate itself would wait better by reporting a BUSY concern than by holding its lane.
	wait_s=${GORGE_TEST_LANE_WAIT:-1500}
	case $wait_s in '' | *[!0-9]*) wait_s=1500 ;; esac
	if [ "$wait_s" -gt 1500 ]; then
		echo "test-targets: note: GORGE_TEST_LANE_WAIT=$wait_s clamped to 1500; on a full heavy lane report the wait as a BUSY concern, do not wait longer" >&2
		wait_s=1500
	fi
	pkgs_label=affected@$aff_sha,dirac-gate@$dg_sha
	echo "gate scripts: gate_affected=$aff_sha dirac-gate=$dg_sha"
elif [ "$smart" = narrow ]; then
	pkgs_label=affected=narrow,dirac-gate=skipped
	echo "gate scripts: gate_affected=narrow dirac-gate=skipped"
else
	pkgs_label=affected=skipped,dirac-gate=skipped
	echo "gate scripts: gate_affected=skipped dirac-gate=skipped"
fi

# A terminal verdict (PASS or FAIL) recorded for this commit with these gate scripts is reused: the commit
# and the gates did not change, so rerunning would repeat the same work. FORCE=1 reruns anyway.
keep=.ds4/scratch/test/smart-${sha%+dirty}
vfile=$keep.verdict
if [ -z "${FORCE:-}" ] && [ -r "$vfile" ]; then
	read -r _ rst _ rpk _ <"$vfile"
	if [ "$rpk" = "$pkgs_label" ] && { [ "$rst" = PASS ] || [ "$rst" = FAIL ]; }; then
		echo "test-targets: note: reused verdict for $sha; make test-smart FORCE=1 to rerun" >&2
		if [ "$rst" = FAIL ]; then
			# the failures behind the verdict, as the run that recorded it printed them
			[ -r "$vfile.detail" ] && { echo "== the failures behind the reused verdict"; cat "$vfile.detail"; }
			for f in "$keep.affected.log" "$keep.dirac.log"; do
				[ -r "$f" ] || continue
				d=$(grep -E -A8 -- '^--- FAIL|^FAIL[[:space:]]|panic:' "$f" | head -40)
				[ -n "$d" ] && { echo "== failures in $f"; echo "$d"; }
			done
		fi
		printf '%s\n' "$(<"$vfile")"
		case $rst in PASS) exit 0 ;; esac
		exit 1
	fi
fi

echo "== 1 gofmt, vet"
files=$(git diff --name-only --diff-filter=d "$mb" HEAD -- '*.go')
if [ -n "$files" ]; then
	unfmt=$(gofmt -l $files)
	[ -z "$unfmt" ] || bad "gofmt: $unfmt"
	vp=$(for f in $files; do echo "./$(dirname "$f")"; done | sort -u)
	vo=$(go vet $vp 2>&1) || { tail -15 <<<"$vo"; bad "go vet"; }
fi
echo "== 2 docs snapshot, history hygiene, oracle skip lint"
python3 -m unittest scripts/docs_sets_test.py >.ds4/scratch/test/docs_sets.log 2>&1 ||
	bad "docs_sets_test: scripts/docs_sets.py reads dirac/docs/support.md; regenerate that page (dirac/LANDGEN) (.ds4/scratch/test/docs_sets.log)"
hyg=$(git log --format='%h %s' "$BASE..HEAD" | grep -iE '^[0-9a-f]+ (wip\b|fixup!|squash!)' | head -3)
zz=$(git diff --name-only --diff-filter=A "$mb" HEAD | grep -E '(^|/)zz_[^/]*$' | head -3)
[ -z "$hyg$zz" ] || bad "history hygiene: reword or squash these commits and delete scratch files: $hyg $zz"
sk=$(git diff -U0 "$mb" HEAD -- 'dirac/oracle/*_test.go' | grep -E '^\+[^+].*\bt\.Skip(f|Now)?\(' | grep -vE 'pool|Pool|tables|corpus' | head -3)
[ -z "$sk" ] || bad "a new t.Skip in dirac/oracle that does not name a missing pool, table or corpus fails CI: $sk"
# CI's lint runs fresh-clone-check: dirac with no corpus, tables or pools, where every corpus-dependent test must skip.
# The landing gate always has the corpus, so a test that fails there reached the promote PR (11-13 PR #328 and #329,
# TestMBCChangeTextReadsStall). About 20 s with a warm cache. A scratch directory with a .cards above it (TMPDIR inside a
# checkout) cannot isolate the tree: that is noted, not failed.
if [ -n "$(git diff --name-only "$mb" HEAD -- 'dirac/' 'engine/' go.mod go.sum)" ]; then
	echo "== 2b fresh clone (dirac with no corpus, as CI's lint)"
	fo=$(capped 4G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=3GiB REF=HEAD bash scripts/fresh-clone-check.sh 2>&1)
	case $fo in
	*"fresh-clone-check: ok"*) ;;
	*"is above the scratch directory"*) echo "test-targets: note: fresh-clone-check skipped: $(grep -m1 'above the scratch' <<<"$fo")" >&2 ;;
	*)
		printf '%s\n' "$fo" >.ds4/scratch/test/fresh-clone.log
		grep -E -- '--- FAIL|^FAIL|panic:|_test\.go:[0-9]+:|fresh-clone-check:' <<<"$fo" | tail -15
		bad "fresh-clone-check failed: a dirac test fails with no corpus (make fresh-clone-check; .ds4/scratch/test/fresh-clone.log)"
		;;
	esac
fi

# oracle_chunked RUN [SKIP]: ./dirac/oracle's tests matching RUN (less SKIP) in fixed chunks of their sorted names,
# each its own go test call with the oracle's cap. 11-20: its census set (~120 tests) and step 4's named set (the
# fast tier and every repro replay) each summed past the 5m cap in one call on a loaded box (0.2-0.7 idle cores),
# so test-smart printed BUDGET for diffs that never touched the oracle. The cmd5 clean-pair ratchet (91-195 s) is
# a chunk of its own. The chunks are the same on every run, so each keeps its test-cache entry.
# ORACLE_CHUNKS round-robin chunks (11-21: 4 still let one chunk pass the 5m cap on a loaded gate box, test-smart
# BUDGET on #3135's diff, which never touched the oracle; each extra chunk costs a few seconds of binary start).
ORACLE_CHUNKS=8
oracle_chunked() {
	local names rx k chunks=() r=0
	names=$(capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -list "$1" ./dirac/oracle 2>/dev/null | grep -E '^Test' | sort) || names=
	[ -z "${2:-}" ] || names=$(grep -vE "$2" <<<"$names" || true)
	if [ -z "$names" ]; then # nothing listed (a stub go, a build error): the one call, as before; it reports why
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "$(timeout_for ./dirac/oracle)" -run "$1" ${2:+-skip "$2"} ./dirac/oracle >>"$json" 2>&1
		return
	fi
	! grep -qx 'TestCmd5SliceCleanOnlyGrows' <<<"$names" || chunks+=('^TestCmd5SliceCleanOnlyGrows$')
	for ((k = 0; k < ORACLE_CHUNKS; k++)); do
		rx=$(grep -vx 'TestCmd5SliceCleanOnlyGrows' <<<"$names" | awk -v k=$k -v n=$ORACLE_CHUNKS 'NR % n == k' | paste -sd'|')
		[ -z "$rx" ] || chunks+=("^($rx)\$")
	done
	for rx in "${chunks[@]}"; do
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "$(timeout_for ./dirac/oracle)" -run "$rx" ./dirac/oracle >>"$json" 2>&1 || { r=$?; break; }
	done
	return $r
}

echo "== 3 census and coverage tests, unrefusal proof"
dirac_tables
C='Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|PublicBoundary|Acceptance|ReprosLockstep|ReproManifest|EnumConst|^TestHeads$|^TestSetHeads$|Pinned$|AreRegistered|ParamsAreRead|FilesWellFormed|KnownKeys|OnlyGrows|^TestKopOrdinalsUnchanged$|^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$'
# The narrow precheck widens it to the audit's test groups (pins, layout, lint, replay) that C's names miss,
# the ones with reds first: the State layout and size, enum values, ParamsAreRead and KnownUnsupported, the
# generated-file freshness pins, the setdata and spellbench support pins, KnownKeys, the zero-alloc pins,
# the kgen pool builds, the refused entries and the findings mirrors. With the gates the dirac and affected
# gates run these.
[ "$smart" != narrow ] || C="$C|StateLayoutPinned|StateSize|ColdArraysHoldNoPointers|EnumValuesUnique|ParamsAreRead|RepoDeckIsFullySupported|KnownUnsupportedFilesWellFormed|IsFresh|IsRendered|UpToDate|SupportHasNoScriptText|SupportEverySheetCard|PendingSetCards|ReportsUnsupportedCards|KnownKeys|AllocationFree|ZeroAlloc|AllocsNothing|PoolBuilds|RefusedEntriesStillDiverge|FindingsMirror"
# The anchored names cannot match inside the Test[A-Za-z0-9_]*($C) grep, so their packages come from their own.
cpk=$({ git grep -lE "^func Test[A-Za-z0-9_]*($C)" -- '*_test.go'; git grep -lE '^func (TestKopOrdinalsUnchanged|TestReproCompactRoundTrip|TestReproGlobMatchesTable)\(' -- '*_test.go'; } | xargs -n1 dirname | sort -u | sed 's|^|./|')
# Only the census packages the change can reach (test_targets.py census: importers of a changed package,
# touched directories, the tree censuses; every one for a change outside any package). A failed selection
# runs them all.
ncpk=$(wc -w <<<"$cpk")
if sel=$(printf '%s\n' $cpk | "${py[@]}" census --base "$BASE") && [ -n "$sel" ]; then cpk=$sel; fi
echo "census packages: $(wc -w <<<"$cpk") of $ncpk"
# ./dirac/oracle's census tests run in calls of their own (below), so it leaves the package groups here.
oracle_census=
if grep -qx './dirac/oracle' <<<"$(printf '%s\n' $cpk)"; then
	oracle_census=1
	cpk=$(printf '%s\n' $cpk | grep -vx './dirac/oracle' || true)
fi
group_by_timeout $cpk ./internal/archtest || finish ""
for i in "${!G_TO[@]}"; do
	read -r -a group <<<"${G_PKGS[i]}"
	capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "${G_TO[i]}" -run "$C" -skip '^TestStd[A-Za-z]*ReprosLockstep' "${group[@]}" >>"$json" 2>&1
	rc=$?
	[ "$rc" = 0 ] || break
done
# ./dirac/oracle's census tests (~120: the repro families, the pins, the cmd5 clean-pair ratchet) summed past the
# oracle's 5m cap in one call on a loaded box (11-20: 264 s done and 7 tests unfinished at 0.2 idle cores, the cmd5
# ratchet alone 174-195 s), so test-smart printed BUDGET on every diff that selected them (any diff outside a Go
# package does). They run in fixed chunks of their sorted names, each its own call with the same cap; the cmd5
# ratchet is a chunk of its own. The chunks are the same on every run, so each keeps its test-cache entry.
if [ "$rc" = 0 ] && [ -n "$oracle_census" ]; then
	oracle_chunked "$C" '^TestStd[A-Za-z]*ReprosLockstep'
	rc=$?
fi
if [ "$rc" = 0 ]; then
	capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB KERNEL_UNREFUSE_BASE="$BASE" go test -json -v -failfast -timeout "$(timeout_for ./dirac/oracle)" -run '^TestUnrefusalNeedsProof$' ./dirac/oracle >>"$json" 2>&1
	rc=$?
fi
[ "$rc" = 0 ] || finish ""

if [ "$smart" = gates ]; then
	echo "== 4 affected gate and dirac gate, side by side"
	lock=${GORGE_HEAVY_LOCK:-}
	lanes=${GORGE_HEAVY_LANES:-3}
	if [ -n "${TEST_TARGETS_REMOTE:-}" ]; then
		: # the box caps its concurrent runs itself (test-remote-box.sh)
	elif [ -n "$lock" ] && { : >>"$lock"; } 2>/dev/null; then
		deadline=$((SECONDS + wait_s))
		got=
		while [ -z "$got" ]; do
			for i in $(seq 1 "$lanes"); do
				f=$lock
				[ "$i" = 1 ] || f=$lock.lane$i
				{ : >>"$f"; } 2>/dev/null || continue
				exec 9>>"$f"
				if flock -n 9; then got=$i; break; fi
			done
			[ -n "$got" ] && break
			[ "$SECONDS" -lt "$deadline" ] || verdict BUSY "no heavy lane in ${wait_s}s; finish the ticket with a BUSY concern instead of retrying with a longer GORGE_TEST_LANE_WAIT"
			sleep 5
		done
	else
		echo "test-targets: note: GORGE_HEAVY_LOCK is unset or not writable; running without a heavy lane" >&2
	fi
	dgs=$(mktemp "${TMPDIR:-/tmp}/test-smart-dirac-gate.XXXXXX") && git show "$BASE:scripts/dirac-gate.sh" >"$dgs" || verdict ENV "cannot read $BASE:scripts/dirac-gate.sh"
	S=$(mktemp -d "${TMPDIR:-/tmp}/test-smart.XXXXXX")
	trap 'rm -rf -- "$S" "$dgs"' EXIT
	ncpu=$(nproc)
	cpu_a=$((ncpu * 100)); [ "$cpu_a" -le 1600 ] || cpu_a=1600
	cpu_d=$((ncpu * 100)); [ "$cpu_d" -le 800 ] || cpu_d=800
	U=tsmart-$$-$(date +%s)
	# The 386 cross-build (TestEngineCompilesFor32Bit, about 90 s) left the dirac gate's step 8 for this, the
	# affected side, which finishes well before the dirac gate (coverage audit 2026-11-14, item 6). It runs
	# when the change can move a GOARCH=386 build: a non-test Go file, go.mod or go.sum.
	A32=
	[ -z "$(git diff --name-only "$mb" HEAD -- '*.go' go.mod go.sum ':(exclude)*_test.go')" ] || A32=1
	export A32
	{
		if [ -n "$have_sd" ]; then
			systemd-run --user --scope -q --unit="$U-aff" -p MemoryMax=16G -p CPUQuota=$cpu_a% env GOFLAGS=-trimpath T="$BASE" A32="$A32" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T" && { [ -z "$A32" ] || go test -timeout 5m -run "^TestEngineCompilesFor32Bit\$" ./internal/archtest; }' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
		else
			GOFLAGS=-trimpath T="$BASE" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T" && { [ -z "$A32" ] || go test -timeout 5m -run "^TestEngineCompilesFor32Bit\$" ./internal/archtest; }' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
		fi
		echo $? >"$S/aff.rc"
	} 9>&- &
	{
		if [ -n "${PRECHECK_DIRAC_GATE:-}" ] && [ -x "$PRECHECK_DIRAC_GATE" ]; then
			"$PRECHECK_DIRAC_GATE" "$BASE" >"$S/dg" 2>&1
		elif [ -n "$have_sd" ]; then
			systemd-run --user --scope -q --unit="$U-dirac" -p MemoryMax=16G -p CPUQuota=$cpu_d% env GOFLAGS=-trimpath GOMAXPROCS=8 GOMEMLIMIT=13GiB bash "$dgs" "$BASE" >"$S/dg" 2>&1
		else
			GOFLAGS=-trimpath bash "$dgs" "$BASE" >"$S/dg" 2>&1
		fi
		echo $? >"$S/dg.rc"
	} 9>&- &
	stopped=
	while :; do
		ra=$(cat "$S/aff.rc" 2>/dev/null)
		rd=$(cat "$S/dg.rc" 2>/dev/null)
		if { [ -n "$ra" ] && [ "$ra" != 0 ]; } || grep -qE '^(--- FAIL|FAIL[[:space:]]|panic:)' "$S/aff" 2>/dev/null; then stopped="affected tests failed"; break; fi
		if [ -n "$rd" ] && [ "$rd" != 0 ]; then stopped="dirac gate failed"; break; fi
		[ -n "$ra" ] && [ -n "$rd" ] && break
		sleep 2
	done
	if [ -n "$stopped" ]; then
		systemctl --user stop "$U-aff.scope" "$U-dirac.scope" >/dev/null 2>&1
		kill $(jobs -p) >/dev/null 2>&1
	fi
	wait
	ra=$(cat "$S/aff.rc" 2>/dev/null)
	rd=$(cat "$S/dg.rc" 2>/dev/null)
	grep -qE '^(--- FAIL|FAIL[[:space:]]|panic:)' "$S/aff" && ra=1
	cp "$S/aff" "$keep.affected.log" 2>/dev/null
	cp "$S/dg" "$keep.dirac.log" 2>/dev/null
	aff_fail=0 dirac_fail=0
	{ [ -z "$ra" ] || [ "$ra" != 0 ]; } && aff_fail=1
	{ [ -z "$rd" ] || [ "$rd" != 0 ]; } && dirac_fail=1
	# Report both gates, as g-land does. The dirac gate often fails first; stopping the affected scope then
	# leaves it at 137/143 (or with no rc file), and that must neither hide the dirac failure nor be read as
	# the budget: the dirac failure decides the landing.
	if [ "$aff_fail" = 1 ]; then
		grep -E -A8 -- '^--- FAIL|^FAIL|panic:' "$S/aff" | head -40
		echo "affected gate rc=${ra:-missing}${stopped:+ ($stopped)}"
		echo "full output: $keep.affected.log"
	fi
	grep -E '^(==|dirac gate|State|Benchmark|fdn:|blb:|std-|  )' "$S/dg" | head -60
	if [ "$dirac_fail" = 1 ]; then
		tail -30 "$S/dg"
		echo "dirac gate rc=${rd:-missing}${stopped:+ ($stopped)}"
		echo "full output: $keep.dirac.log"
		bad "dirac gate rc=${rd:-missing}; affected gate rc=${ra:-missing}${stopped:+ ($stopped)}"
	fi
	if [ "${ra:-}" = 137 ] || [ "${ra:-}" = 143 ]; then
		[ -z "$fbcap" ] || verdict BUSY "affected gate killed (rc=$ra) at the fallback cap $fbcap: the machine's, not the change's"
		verdict BUDGET "affected gate killed (rc=$ra)"
	fi
	[ "$aff_fail" = 0 ] || bad "affected gate rc=${ra:-missing}${stopped:+ ($stopped)}"

	if grep -q 'dirac gate: skipped' "$S/dg"; then
		echo "== 4b repro tables (the dirac gate skipped)"
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "$(timeout_for ./dirac/oracle)" -run '^TestStd[A-Za-z]*ReprosLockstep' ./dirac/oracle >>"$json" 2>&1
		rc=$?
		[ "$rc" = 0 ] || finish ""
	fi
elif [ "$smart" = narrow ]; then
	echo "== 4 affected packages and the lint packages (the prescreen and the lander run the affected and dirac gates)"
	sel=$("${py[@]}" select --base "$BASE" --narrow) || bad "selection failed: $sel"
	whole=() runs=() named=()
	while IFS=$'\t' read -r kind pkg rx; do
		[ -n "$kind" ] || continue
		case $kind in
		whole) whole+=("$pkg"); echo "   whole  $pkg" ;;
		wholeskip) runs+=("skip"$'\t'"$pkg"$'\t'"$rx"); echo "   whole  $pkg (-skip $(cut -c1-80 <<<"$rx"))" ;;
		named) runs+=("run"$'\t'"$pkg"$'\t'"$rx"); named+=("$mod/${pkg#./}"); echo "   named  $pkg" ;;
		census) runs+=("run"$'\t'"$pkg"$'\t'"$rx"); echo "   named  $pkg" ;;
		excluded) echo "test-targets: note: $pkg imports a changed package but is over budget; it runs post-merge" >&2 ;;
		esac
	done <<<"$sel"
	: >"$json"
	rc=0
	# The named runs first, then the light whole packages together, then ./engine/rules and archtest alone.
	for r in "${runs[@]+"${runs[@]}"}"; do
		IFS=$'\t' read -r how pkg rx <<<"$r"
		[ "$how" = run ] || continue
		if [ "$pkg" = ./dirac/oracle ]; then oracle_chunked "$rx" || { rc=$?; finish ""; }; continue; fi
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "$(timeout_for "$pkg")" -run "$rx" "$pkg" >>"$json" 2>&1 || { rc=$?; finish ""; }
	done
	if [ "${#whole[@]}" -gt 0 ]; then
		group_by_timeout "${whole[@]}" || finish ""
		for i in "${!G_TO[@]}"; do
			read -r -a group <<<"${G_PKGS[i]}"
			capped 12G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=3GiB go test -json -v -failfast -timeout "${G_TO[i]}" -p "$lp" "${group[@]}" >>"$json" 2>&1 || { rc=$?; finish ""; }
		done
	fi
	for r in "${runs[@]+"${runs[@]}"}"; do
		IFS=$'\t' read -r how pkg rx <<<"$r"
		[ "$how" = skip ] || continue
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=3GiB go test -json -v -failfast -timeout "$(timeout_for "$pkg")" -skip "$rx" "$pkg" >>"$json" 2>&1 || { rc=$?; finish ""; }
	done
	# A named run that matched nothing is EMPTY, not a pass: classify the stream before step 5 replaces it.
	nm=$(IFS=,; echo "${named[*]-}")
	"${py[@]}" finish --mode "$mode" --json "$json" --rc 0 --sha "$sha" --pkgs "$pkgs_label" --seconds 0 --named "$nm" --base "$BASE" ${fbcap:+--fallback "$fbcap"} >/dev/null || finish "$nm"
else
	echo "== 4 skipped (GORGE_TEST_SMART_GATES=0: the lander runs the affected and dirac gates)"
	if [ -n "$(git diff --name-only "$mb" HEAD -- 'dirac/oracle/testdata/repro/')" ]; then
		echo "== 4b repro tables (the diff touches dirac/oracle/testdata/repro/)"
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -v -failfast -timeout "$(timeout_for ./dirac/oracle)" -run '^TestStd[A-Za-z]*ReprosLockstep' ./dirac/oracle >>"$json" 2>&1
		rc=$?
		[ "$rc" = 0 ] || finish ""
	fi
fi

echo "== 5 wire goldens"
: >"$json"
rc=0
gojson "$(timeout_for ./server/gorged)" -run '^TestNativeWireGolden$' ./server/gorged
finish "$mod/server/gorged"
