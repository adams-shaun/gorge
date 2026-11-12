#!/usr/bin/env bash
# test-targets.sh <one|targeted|smart|list>: the make targets that replace a raw `go test`.
#
#   make test-one PKG=./p RUN='^TestX$'   one package, named tests: debugging, regeneration, scratch probes
#   make test-targeted [BASE=land]        packages the diff touches, plus every package whose tests import them
#   make test-smart [BASE=land]           what the landing gate runs, fail-fast, on a committed tree
#   make precheck                         an alias for test-smart
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
# GORGE_HEAVY_LOCK (+ GORGE_HEAVY_LANES, GORGE_TEST_LANE_WAIT).
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

# verdict STATUS [msg]: print the verdict line and leave with its exit code.
verdict() {
	local st=$1 msg=${2:-}
	"${py[@]}" verdict --mode "$mode" --status "$st" --sha "$sha" --pkgs "$pkgs_label" --seconds $((SECONDS - t0)) ${msg:+--msg "$msg"}
	exit $?
}
usage() { echo "test-targets: $mode: $1" >&2; exit 2; }

# ---- the cap -----------------------------------------------------------------------------------------
if [ -z "${TEST_TARGETS_SCOPED:-}" ]; then
	export GORGE_TEST_WRAPPER=1
	mem=4G lim=3GiB cpu=400%
	if [ "$mode" = targeted ] || [ "${PKG:-}" = ./dirac/oracle ]; then mem=8G lim=6GiB; fi
	if [ "$mode" = list ] || [ "$mode" = smart ]; then
		: # list runs go list only; smart caps each of its own steps
	elif systemd-run --user --scope -q true >/dev/null 2>&1; then
		exec systemd-run --user --scope -q -p MemoryMax=$mem -p CPUQuota=$cpu env TEST_TARGETS_SCOPED=1 GOMAXPROCS=4 GOMEMLIMIT=$lim bash "$top/scripts/test-targets.sh" "$@"
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
systemd-run --user --scope -q true >/dev/null 2>&1 && have_sd=1
capped() {
	if [ -n "$have_sd" ]; then systemd-run --user --scope -q -p MemoryMax="$1" -p CPUQuota="$2" "${@:3}"; else "${@:3}"; fi
}
dirac_tables() { # needed before any dirac package plays a game; a no-op when the tables are current
	local c=(); [ -n "${TEST_TARGETS_SCOPED:-}" ] || c=(capped 8G 400% env GOMAXPROCS=4 GOMEMLIMIT=6GiB)
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
	"${py[@]}" finish --mode "$mode" --json "$json" --rc "$rc" --sha "$sha" --pkgs "$pkgs_label" --seconds $((SECONDS - t0)) --named "$1"
	exit $?
}

# ---- test-one ----------------------------------------------------------------------------------------
if [ "$mode" = one ]; then
	pkg=${PKG:-}
	[[ $pkg =~ ^\./[A-Za-z0-9_./-]+$ && $pkg != *...* ]] || usage "PKG=./path/to/one/package is required (got '$pkg'); no ./... and no wildcards"
	[ -n "${RUN:-}" ] || usage "RUN=<test regexp> is required"
	case $pkg in ./engine/rules | ./dirac/oracle)
		case ${RUN} in . | .\* | ^) usage "$pkg is too big to run whole: name the tests in RUN" ;; esac ;;
	esac
	to=${TIMEOUT:-2m}
	[[ $to =~ ^([0-9]+)([sm])$ ]] || usage "TIMEOUT is Ns or Nm"
	secs=${BASH_REMATCH[1]}
	[ "${BASH_REMATCH[2]}" = s ] || secs=$((secs * 60))
	[ "$secs" -le 300 ] || usage "TIMEOUT is at most 5m"
	[ -z "${TAGS:-}" ] || [[ $TAGS =~ ^[A-Za-z0-9_,]+$ ]] || usage "TAGS is a comma list of build tags"
	envs=()
	for kv in ${TESTENV:-}; do
		case ${kv%%=*} in KGEN_REPIN | DIRAC_UPDATE_SUPPORT | KOP_ORDINALS_REGEN | PINS_RECORD | GORGE_POOLS) envs+=("$kv") ;;
		*) usage "TESTENV allows only KGEN_REPIN DIRAC_UPDATE_SUPPORT KOP_ORDINALS_REGEN PINS_RECORD GORGE_POOLS (got ${kv%%=*})" ;;
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
		gojson 10m -p 4 "${whole[@]}" || finish ""
	fi
	for r in "${runs[@]+"${runs[@]}"}"; do
		gojson 5m -run "${r#*$'\t'}" "${r%%$'\t'*}" || finish ""
	done
	finish "$(IFS=,; echo "${named[*]-}")"
fi

# ---- test-smart --------------------------------------------------------------------------------------
# The gate's own steps, in its order, fail-fast (fleet/land/tools/wland2.sh and g-land.sh): gofmt, the docs
# snapshot, history hygiene, the oracle skip lint; the census tests; then the affected-package gate
# (the affected-package gate script, read from GATE_AFFECTED_REPO's GATE_REF) and the dirac gate (BASE's scripts/dirac-gate.sh) side by
# side in a heavy lane; then the wire goldens. precheck.sh's census and unrefusal steps stay in, so the
# alias loses nothing.
[ -z "$(git status --porcelain --untracked-files=no)" ] || verdict ENV "commit first: test-smart checks a commit, not a dirty tree"
# The affected-package gate lives in another repository; the operator's env file names it.
GATE_AFFECTED_PATH=${GATE_AFFECTED_PATH:-ops/scripts/gate_affected.sh}
GATE_REF=${GATE_REF:-main}
[ -n "${GATE_AFFECTED_REPO:-}" ] || verdict ENV "GATE_AFFECTED_REPO is not set: the git repository that holds $GATE_AFFECTED_PATH (set it in $envfile)"
aff_sha=$(git -C "$GATE_AFFECTED_REPO" rev-parse --short=7 "$GATE_REF:$GATE_AFFECTED_PATH" 2>/dev/null) ||
	verdict ENV "$GATE_AFFECTED_PATH not found in $GATE_AFFECTED_REPO at $GATE_REF: set GATE_AFFECTED_REPO, GATE_AFFECTED_PATH, GATE_REF"
dg_sha=$(git rev-parse --short=7 "$BASE:scripts/dirac-gate.sh" 2>/dev/null) || verdict ENV "$BASE has no scripts/dirac-gate.sh"
pkgs_label=affected@$aff_sha,dirac-gate@$dg_sha
echo "gate scripts: gate_affected=$aff_sha dirac-gate=$dg_sha"
mb=$(git merge-base "$BASE" HEAD)
bad() { echo "test-targets: smart: $*" >&2; verdict FAIL "$*"; }

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
C='Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|PublicBoundary|Acceptance|ReprosLockstep|EnumConst|^TestHeads$|^TestSetHeads$'
cpk=$(git grep -lE "^func Test[A-Za-z0-9_]*($C)" -- '*_test.go' | xargs -n1 dirname | sort -u | sed 's|^|./|')
capped 8G 400% env GOMAXPROCS=4 GOMEMLIMIT=6GiB go test -json -failfast -timeout 10m -run "$C" -skip '^TestStd[A-Za-z]*ReprosLockstep' $cpk ./internal/archtest >>"$json" 2>&1
rc=$?
if [ "$rc" = 0 ]; then
	capped 8G 400% env GOMAXPROCS=4 GOMEMLIMIT=6GiB KERNEL_UNREFUSE_BASE="$BASE" go test -json -failfast -timeout 5m -run '^TestUnrefusalNeedsProof$' ./dirac/oracle >>"$json" 2>&1
	rc=$?
fi
[ "$rc" = 0 ] || finish ""

echo "== 4 affected gate and dirac gate, side by side"
lock=${GORGE_HEAVY_LOCK:-}
lanes=${GORGE_HEAVY_LANES:-3}
wait_s=${GORGE_TEST_LANE_WAIT:-1500}
if [ -n "$lock" ] && [ -d "$(dirname "$lock")" ]; then
	deadline=$((SECONDS + wait_s))
	got=
	while [ -z "$got" ]; do
		for i in $(seq 1 "$lanes"); do
			f=$lock
			[ "$i" = 1 ] || f=$lock.lane$i
			: >>"$f"
			exec 9>>"$f"
			if flock -n 9; then got=$i; break; fi
		done
		[ -n "$got" ] && break
		[ "$SECONDS" -lt "$deadline" ] || verdict BUSY "no heavy lane in ${wait_s}s"
		sleep 5
	done
else
	echo "test-targets: note: GORGE_HEAVY_LOCK is unset or its directory is missing; running without a heavy lane" >&2
fi
dgs=$(mktemp "${TMPDIR:-/tmp}/test-smart-dirac-gate.XXXXXX") && git show "$BASE:scripts/dirac-gate.sh" >"$dgs" || verdict ENV "cannot read $BASE:scripts/dirac-gate.sh"
S=$(mktemp -d "${TMPDIR:-/tmp}/test-smart.XXXXXX")
trap 'rm -rf -- "$S" "$dgs"' EXIT
ncpu=$(nproc)
cpu_a=$((ncpu * 100)); [ "$cpu_a" -le 1600 ] || cpu_a=1600
cpu_d=$((ncpu * 100)); [ "$cpu_d" -le 800 ] || cpu_d=800
U=tsmart-$$-$(date +%s)
{
	if systemd-run --user --scope -q true >/dev/null 2>&1; then
		systemd-run --user --scope -q --unit="$U-aff" -p MemoryMax=16G -p CPUQuota=$cpu_a% env GOFLAGS=-trimpath T="$BASE" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T"' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
	else
		GOFLAGS=-trimpath T="$BASE" bash -c 'set -o pipefail; git -C "$0" show "$1:$2" | bash -s "$T"' "$GATE_AFFECTED_REPO" "$GATE_REF" "$GATE_AFFECTED_PATH" >"$S/aff" 2>&1
	fi
	echo $? >"$S/aff.rc"
} 9>&- &
{
	if [ -n "${PRECHECK_DIRAC_GATE:-}" ] && [ -x "$PRECHECK_DIRAC_GATE" ]; then
		"$PRECHECK_DIRAC_GATE" "$BASE" >"$S/dg" 2>&1
	elif systemd-run --user --scope -q true >/dev/null 2>&1; then
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
keep=.ds4/scratch/test/smart-${sha%+dirty}
cp "$S/aff" "$keep.affected.log" 2>/dev/null
cp "$S/dg" "$keep.dirac.log" 2>/dev/null
case ${ra:-1} in 137 | 143) echo "affected gate killed (rc=$ra)"; verdict BUDGET "affected gate killed" ;; esac
if [ "${ra:-1}" != 0 ]; then
	grep -E -A8 -- '^--- FAIL|^FAIL|panic:' "$S/aff" | head -40
	echo "full output: $keep.affected.log"
	bad "affected gate rc=${ra:-1}${stopped:+ ($stopped)}"
fi
grep -E '^(==|dirac gate|State|Benchmark|fdn:|blb:|std-|  )' "$S/dg" | head -60
if [ "${rd:-1}" != 0 ]; then
	tail -30 "$S/dg"
	echo "full output: $keep.dirac.log"
	bad "dirac gate rc=${rd:-1}${stopped:+ ($stopped)}"
fi

echo "== 5 wire goldens"
: >"$json"
rc=0
gojson 3m -run '^TestNativeWireGolden$' ./server/gorged
finish "$mod/server/gorged"
