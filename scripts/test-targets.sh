#!/usr/bin/env bash
# test-targets.sh <one|targeted|smart|list>: the make targets that replace a raw `go test`.
#
#   make test-one PKG=./p RUN='^TestX$'   one package, named tests: debugging, regeneration, scratch probes
#   make test-targeted [BASE=land]        packages the diff touches, plus every package whose tests import them
#   make test-smart [BASE=land]           the steps a seat can act on, fail-fast, on a committed tree
#   make precheck                         an alias for test-smart
#
# GORGE_TEST_SMART_GATES=1 adds the affected-package and dirac gates to test-smart; by default the
# lander runs them, so a seat does not pay for them twice.
#
# Each run sits in one user systemd scope (MemoryMax, CPUQuota, GOMAXPROCS, GOMEMLIMIT) and exports
# GORGE_TEST_WRAPPER=1. With no user systemd it says so and runs uncapped.
# Preflight: the .cards corpus resolves and is not empty, dirac's tables exist when a dirac package runs,
# BASE resolves. Postflight (scripts/test_targets.py): a skip that names the corpus, pools or tables is ENV,
# a -run that matched nothing is EMPTY, a timeout or kill is BUDGET.
#
# The last line on stdout is the one verdict line, and nothing else printed starts with "test-":
#
#   test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>
#
# Settings that belong to the machine, not the repository, go in ~/.config/gorge/test-targets.env (or
# $GORGE_TEST_TARGETS_ENV): GORGE_POOLS (dirac's deck pools), GATE_AFFECTED_REPO (+ GATE_AFFECTED_PATH, GATE_REF),
# GORGE_HEAVY_LOCK (+ GORGE_HEAVY_LANES, GORGE_TEST_LANE_WAIT), and the remote runner's GORGE_REMOTE_HOST,
# GORGE_REMOTE_KEY, GORGE_REMOTE_REPO, GORGE_REMOTE_MIN_IDLE, GORGE_REMOTE_OFF (scripts/test-remote.sh).
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
BASE=${BASE:-land}
t0=$SECONDS
sha=$(git rev-parse --short=10 HEAD 2>/dev/null || echo -)
[ -z "$(git status --porcelain --untracked-files=no 2>/dev/null)" ] || sha=$sha+dirty
pkgs_label=""
# shellcheck disable=SC1091
. "$top/scripts/test-remote.sh"

# record_verdict STATUS LINE: remember a terminal verdict (PASS or FAIL) for the commit, next to the logs, so
# a rerun on the same commit with the same gate scripts reuses it instead of repeating the work. ENV, EMPTY,
# BUDGET and BUSY describe the box, not the commit, and are never recorded.
record_verdict() {
	case $1 in
	PASS | FAIL) printf '%s\n' "$2" >.ds4/scratch/test/smart-${sha%+dirty}.verdict ;;
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

# ---- the cap -----------------------------------------------------------------------------------------
if [ -z "${TEST_TARGETS_SCOPED:-}" ]; then
	export GORGE_TEST_WRAPPER=1
	mem=4G lim=3GiB cpu=400% procs=4
	if [ "$mode" = targeted ] || [ "${PKG:-}" = ./dirac/oracle ]; then mem=8G lim=6GiB; fi
	# A run that fell back from the configured gate box: the box is down, so every seat's heavy run lands here at
	# once; cap it lower (test-remote.sh, GORGE_REMOTE_FALLBACK_CPU).
	[ -z "${TEST_TARGETS_FALLBACK:-}" ] || { cpu=$fcpu%; procs=$fprocs; }
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
	verdict ENV "no corpus: $cards/cardsfolder is missing or empty, so every corpus test would skip. ln -s <main checkout>/.cards .cards (or make link-cards)"
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
rc=0
gojson() {
	local to=$1
	shift
	go test -json -failfast -timeout "$to" "$@" >>"$json" 2>&1
	local r=$?
	[ "$r" -le "$rc" ] || rc=$r
	return $r
}
finish() { # finish NAMED_IMPORT_PATHS: classify $json, print detail and the verdict, exit
	local out pyrc vline st
	out=$("${py[@]}" finish --mode "$mode" --json "$json" --rc "$rc" --sha "$sha" --pkgs "$pkgs_label" --seconds $((SECONDS - t0)) --named "$1")
	pyrc=$?
	printf '%s\n' "$out"
	if [ "$mode" = smart ]; then
		vline=${out##*$'\n'}
		st=${vline#test-smart: }
		st=${st%% *}
		record_verdict "$st" "$vline"
	fi
	exit $pyrc
}

# ---- test-one ----------------------------------------------------------------------------------------
if [ "$mode" = one ]; then
	pkg=${PKG:-}
	[[ $pkg =~ ^\./[A-Za-z0-9_./-]+$ && $pkg != *...* ]] || usage "PKG=./path/to/one/package is required (got '$pkg'); no ./... and no wildcards"
	[ -n "${RUN:-}" ] || usage "RUN=<test regexp> is required"
	case $pkg in ./engine/rules | ./dirac/oracle)
		# An exact name (or a |-alternation of them), so a broad pattern cannot run the whole package.
		[[ $RUN =~ ^\^[A-Za-z0-9_|^$]*[A-Za-z0-9_]\$$ ]] ||
			usage "$pkg is too big to run whole: name the exact tests in RUN (e.g. '^TestX\$'); got '$RUN'"
		;;
	esac
	to=${TIMEOUT:-2m}
	[[ $to =~ ^([0-9]+)([sm])$ ]] || usage "TIMEOUT is Ns or Nm"
	secs=${BASH_REMATCH[1]}
	[ "${BASH_REMATCH[2]}" = s ] || secs=$((secs * 60))
	[ "$secs" -ge 1 ] && [ "$secs" -le 300 ] || usage "TIMEOUT is between 1s and 5m"
	[ -z "${TAGS:-}" ] || [[ $TAGS =~ ^[A-Za-z0-9_,]+$ ]] || usage "TAGS is a comma list of build tags"
	envs=()
	for kv in ${TESTENV:-}; do
		case ${kv%%=*} in KGEN_REPIN | DIRAC_UPDATE_SUPPORT | KOP_ORDINALS_REGEN | PINS_RECORD | CMD5_CLEAN_REPIN | GORGE_POOLS) envs+=("$kv") ;;
		*) usage "TESTENV allows only KGEN_REPIN DIRAC_UPDATE_SUPPORT KOP_ORDINALS_REGEN PINS_RECORD CMD5_CLEAN_REPIN GORGE_POOLS (got ${kv%%=*})" ;;
		esac
	done
	pkgs_label=$pkg
	case $pkg in ./dirac*) dirac_tables ;; esac
	args=(-run "$RUN")
	[ -z "${SKIP:-}" ] || args+=(-skip "$SKIP")
	[ -z "${TAGS:-}" ] || args+=(-tags "$TAGS")
	[ -z "${V:-}" ] || args+=(-v)
	args+=("$pkg")
	[ -z "${UPDATE:-}" ] || args+=(-args -update)
	env ${envs[@]+"${envs[@]}"} go test -json -failfast -timeout "$to" "${args[@]}" >>"$json" 2>&1
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
	if [ "${#whole[@]}" -gt 0 ]; then
		gojson 10m -p "$lp" "${whole[@]}" || finish ""
	fi
	for r in "${runs[@]+"${runs[@]}"}"; do
		gojson 5m -run "${r#*$'\t'}" "${r%%$'\t'*}" || finish ""
	done
	finish "$(IFS=,; echo "${named[*]-}")"
fi

# ---- test-smart --------------------------------------------------------------------------------------
# The steps a seat can act on, fail-fast (fleet/land/tools/wland2.sh and g-land.sh): gofmt, the docs
# snapshot, history hygiene, the oracle skip lint; the census tests; the repro tables when the diff touches
# dirac/oracle/testdata/repro/; then the wire goldens. The affected-package gate (GATE_AFFECTED_REPO's
# GATE_REF) and the dirac gate (BASE's scripts/dirac-gate.sh), side by side in a heavy lane, are the lander's
# job: GORGE_TEST_SMART_GATES=1 opts back in to them, and then the repro tables also run when the dirac gate
# skipped. precheck.sh's census, unrefusal and 3b repro steps stay in, so the alias loses nothing. Not run
# here (follow-ups in the report): wland2's pinrise re-pin and g-land's remote/farm (rgate) path.
[ -z "$(git status --porcelain --untracked-files=no)" ] || verdict ENV "commit first: test-smart checks a commit, not a dirty tree"
mb=$(git merge-base "$BASE" HEAD)
bad() { echo "test-targets: smart: $*" >&2; verdict FAIL "$*"; }
if [ "${GORGE_TEST_SMART_GATES:-}" = 1 ]; then
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
	bad "docs_sets_test: python3 scripts/docs_sets.py --refresh-dirac docs/site/support/dirac-sets.json --commit <base> --date <today> (.ds4/scratch/test/docs_sets.log)"
hyg=$(git log --format='%h %s' "$BASE..HEAD" | grep -iE '^[0-9a-f]+ (wip\b|fixup!|squash!)' | head -3)
zz=$(git diff --name-only --diff-filter=A "$BASE" HEAD | grep -E '(^|/)zz_[^/]*$' | head -3)
[ -z "$hyg$zz" ] || bad "history hygiene: reword or squash these commits and delete scratch files: $hyg $zz"
sk=$(git diff -U0 "$BASE" HEAD -- 'dirac/oracle/*_test.go' | grep -E '^\+[^+].*\bt\.Skip(f|Now)?\(' | grep -vE 'pool|Pool|tables|corpus' | head -3)
[ -z "$sk" ] || bad "a new t.Skip in dirac/oracle that does not name a missing pool, table or corpus fails CI: $sk"

echo "== 3 census and coverage tests, unrefusal proof"
dirac_tables
C='Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|PublicBoundary|Acceptance|ReprosLockstep|EnumConst|^TestHeads$|^TestSetHeads$|^TestKopOrdinalsUnchanged$|^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$'
# The anchored names cannot match inside the Test[A-Za-z0-9_]*($C) grep, so their packages come from their own.
cpk=$({ git grep -lE "^func Test[A-Za-z0-9_]*($C)" -- '*_test.go'; git grep -lE '^func (TestKopOrdinalsUnchanged|TestReproCompactRoundTrip|TestReproGlobMatchesTable)\(' -- '*_test.go'; } | xargs -n1 dirname | sort -u | sed 's|^|./|')
# Only the census packages the change can reach (test_targets.py census: importers of a changed package,
# touched directories, the tree censuses; every one for a change outside any package). A failed selection
# runs them all.
ncpk=$(wc -w <<<"$cpk")
if sel=$(printf '%s\n' $cpk | "${py[@]}" census --base "$BASE") && [ -n "$sel" ]; then cpk=$sel; fi
echo "census packages: $(wc -w <<<"$cpk") of $ncpk"
capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -failfast -timeout 10m -run "$C" -skip '^TestStd[A-Za-z]*ReprosLockstep' $cpk ./internal/archtest >>"$json" 2>&1
rc=$?
if [ "$rc" = 0 ]; then
	capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB KERNEL_UNREFUSE_BASE="$BASE" go test -json -failfast -timeout 5m -run '^TestUnrefusalNeedsProof$' ./dirac/oracle >>"$json" 2>&1
	rc=$?
fi
[ "$rc" = 0 ] || finish ""

if [ "${GORGE_TEST_SMART_GATES:-}" = 1 ]; then
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
	{
		if [ -n "$have_sd" ]; then
			systemd-run --user --scope -q --unit="$U-aff" -p MemoryMax=16G -p CPUQuota=$cpu_a% env GOFLAGS=-trimpath T="$BASE" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T"' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
		else
			GOFLAGS=-trimpath T="$BASE" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T"' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
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
		verdict BUDGET "affected gate killed (rc=$ra)"
	fi
	[ "$aff_fail" = 0 ] || bad "affected gate rc=${ra:-missing}${stopped:+ ($stopped)}"

	if grep -q 'dirac gate: skipped' "$S/dg"; then
		echo "== 4b repro tables (the dirac gate skipped)"
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -failfast -timeout 10m -run '^TestStd[A-Za-z]*ReprosLockstep' ./dirac/oracle >>"$json" 2>&1
		rc=$?
		[ "$rc" = 0 ] || finish ""
	fi
else
	echo "== 4 skipped (the lander runs the affected and dirac gates)"
	if [ -n "$(git diff --name-only "$mb" HEAD -- 'dirac/oracle/testdata/repro/')" ]; then
		echo "== 4b repro tables (the diff touches dirac/oracle/testdata/repro/)"
		capped 8G "$lcpu" env GOMAXPROCS="$lprocs" GOMEMLIMIT=6GiB go test -json -failfast -timeout 10m -run '^TestStd[A-Za-z]*ReprosLockstep' ./dirac/oracle >>"$json" 2>&1
		rc=$?
		[ "$rc" = 0 ] || finish ""
	fi
fi

echo "== 5 wire goldens"
: >"$json"
rc=0
gojson 3m -run '^TestNativeWireGolden$' ./server/gorged
finish "$mod/server/gorged"
