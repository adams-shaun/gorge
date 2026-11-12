#!/usr/bin/env bash
# dirac-gate.sh <base> -- what a change to dirac, or to anything dirac's oracle plays against,
# must keep true. Every comparison is with the merge base. A pipeline reads this file from <base>
# (`git show <base>:scripts/dirac-gate.sh | bash -s <base>`), so a branch cannot weaken the gate
# that judges it.
#
#   0. protected: no edit to this file, to maxStateBytes or to the evidence constants, and no card
#      leaves the evidence lists unless the change refuses it.
#   1. path filter: skipped when no changed file is under dirac/ (its docs aside) or in kfuzz's
#      dependency closure and FORGE_REF did not move; test files do not count.
#   2. layout: unsafe.Sizeof(State{}) stays under the base's maxStateBytes.
#   3. allocs: BenchmarkTransitionFDN stays at 0 allocs/op; wall time against the base is advisory,
#      since it is noise on a loaded machine and step 5 is the deterministic measure.
#   4. lockstep: kfuzz -absolute on the committed FDN and BLB slices, both surfaces. A divergence,
#      an unmodelled stall or a hang fails; a capacity stall is counted and reported.
#   5. instructions: per-op instructions:u of the transition benches, perf stat under GOGC=off,
#      differenced (N500k-N100k)/400000. FAIL above INSTR_MB_MAX times the merge base, or for FDN
#      and BLB above INSTR_PB_MAX times the accepted program base below.
#   6. inlining: the hot accessors stay inlinable.
#   7. escapes: no dirac local moves to the heap.
#   8. tests: ./dirac, ./dirac/crcoverage, internal/archtest; and, whatever the change touched,
#      every refused entry still diverges, every committed repro table (TestStd*ReprosLockstep)
#      plays in lockstep, and every pinned game still reaches what its pin is named for: the
#      data-driven pins in dirac/oracle/testdata/pins play under TestPinManifest (one subtest per
#      pin), and the pins a Go test still drives by their file names (<test>.<n>.json, a subtest's
#      "/" written "__"), about 35 s together: a gorge fix can break any of them as easily as a
#      dirac one.
#   9. synthetic pools: a short kfuzz -synth on the cards the slices of step 4 don't reach. A pair
#      divergent on the merge base and the head is an open divergence, counted and not failed; a
#      pair newly divergent fails.
#  mp. the dirac_mp build (make dirac-mp-check: vet, the State bar and layout pin, then lockstep on
#      4-seat vanilla pods and the FDN and BLB slices at two seats), about 60 s, only when a non-test
#      file under dirac/ changed. A failure fails the gate.
#
# Steps 2-3 and 5-7 and step 8's package tests run only when a non-test file under dirac/ changed. A
# merge base without dirac/ has nothing to compare, so the base rows print a note. The steps run
# cheapest first (0-3, 6-8, then 4, mp, 5 and 9), and the first failure ends the run.
#
# A commit's measurements (State size, the FDN bench line, instructions per bench, each synthetic
# pool's divergent pairs) are cached under $DIRAC_GATE_CACHE, keyed by the commit, the corpus digest
# and the knobs that shape them. A landing's head is the next landing's merge base, so the base side
# is read from the cache and the base worktree is built only on a miss. Steps 4 and 9 run their
# kfuzz replays concurrently; each pair's result does not depend on what else runs.
set -uo pipefail
export LC_ALL=C # comm and sort must agree on the order of card names
base=${1:?usage: dirac-gate.sh <base>}
INSTR_MB_MAX=${INSTR_MB_MAX:-1.05}
INSTR_PB_MAX=${INSTR_PB_MAX:-1.02}
INSTR_CORE=${INSTR_CORE:-20}
PERF_MAX=${PERF_MAX:-1.06}
# The accepted program base: instructions per op before the instruction budget was set.
INSTR_PB_FDN=52373
INSTR_PB_BLB=54677

top=$(git rev-parse --show-toplevel) || exit 2
cd "$top" || exit 2
mb=$(git merge-base "$base" HEAD) || exit 2
changed=$(git diff --name-only "$mb" HEAD)
fail=0
# The first failure ends the run (operator, 2026-11-04): a red step decides the landing, and only the
# develop->main gates run to completion.
bad() { echo "dirac gate: FAIL -- $*"; exit 1; }

echo "== 0 protected"
# maxStateBytes lives in a test file and an evidence list in no package, so this runs before the
# path filter can skip them.
have_base=
git cat-file -e "$mb:dirac/kernel_test.go" 2>/dev/null && have_base=1
prot=""
if git cat-file -e "$mb:scripts/dirac-gate.sh" 2>/dev/null && grep -qx 'scripts/dirac-gate.sh' <<<"$changed"; then prot="scripts/dirac-gate.sh"; fi
if [ -n "$have_base" ]; then
	git diff "$mb" HEAD -- dirac/kernel_test.go | grep -qE '^[-+].*maxStateBytes' && prot="$prot maxStateBytes"
	git diff "$mb" HEAD -- dirac/oracle/evidence.go | grep -qE '^[-+].*(Evidence[A-Z]|Synth[A-Z]|TailBucket)' && prot="$prot evidence constants"
fi
[ -z "$prot" ] || bad "edits what only the maintainers change: $prot"
evidence_at() { git grep -h '' "$1" -- dirac/support/evidence | sort -u; }
refused_at() { git grep -h '"card":' "$1" -- dirac/support/refused | sed -E 's/.*"card": "(.*)",?$/\1/' | sort -u; }
dropped=$(comm -23 <(evidence_at "$mb") <(evidence_at HEAD) | comm -23 - <(refused_at HEAD))
[ -z "$dropped" ] || bad "cards leave the evidence lists with no refusal: $(tr '\n' ',' <<<"$dropped")"

echo "== 1 path filter"
closure=$(go list -deps -f '{{if not .Standard}}{{.Dir}}{{end}}' ./dirac/oracle/cmd/kfuzz) || { echo "dirac gate: go list failed (infrastructure)"; exit 2; }
closure=$(sed "s|^$top/||" <<<"$closure" | sort -u)
in_closure=$(while IFS= read -r f; do
	case $f in *_test.go|''|dirac/docs/*|dirac/README.md|dirac/PROVENANCE) continue ;; dirac/*) printf '%s\n' "$f"; continue ;; esac
	if grep -qxF "$(dirname "$f")" <<<"$closure"; then printf '%s\n' "$f"; fi
done <<<"$changed")
forge_ref_moved=
if grep -qx Makefile <<<"$changed" && git diff "$mb" HEAD -- Makefile | grep -qE '^[-+]FORGE_REF'; then forge_ref_moved=1; fi
if [ -z "$in_closure" ] && [ -z "$forge_ref_moved" ]; then
	[ "$fail" = 0 ] || exit 1
	echo "dirac gate: skipped (no file in kfuzz's dependency closure)"
	exit 0
fi
dirac_changed=$(grep -E '^dirac/' <<<"$changed" | grep -v '_test\.go$')

S=$(mktemp -d "${TMPDIR:-/tmp}/dirac-gate.XXXXXX")
export GORGE_CARDS=${GORGE_CARDS:-$top/.cards}
[ -d "$GORGE_CARDS/cardsfolder" ] || { echo "dirac gate: no corpus at $GORGE_CARDS: every lockstep game would skip (infrastructure)"; exit 2; }
trap 'git -C "$top" worktree remove --force "$S/base" >/dev/null 2>&1; rm -rf -- "$S"' EXIT

make dirac-tables >"$S/tables.log" 2>&1 || { tail -20 "$S/tables.log"; bad "make dirac-tables failed"; exit 1; }
go build -o "$S/kfuzz" ./dirac/oracle/cmd/kfuzz || { bad "kfuzz does not build"; exit 1; }

synth_pools=${GATE_SYNTH_POOLS:-std-blb std-otj std-mkm}
synth_pairs=${GATE_SYNTH_PAIRS:-300}
head=$(git rev-parse HEAD)
dirty=$(git status --porcelain --untracked-files=no)
corpus=$(sed -nE 's/.*"digest": *"([0-9a-f]{16}).*/\1/p' "$GORGE_CARDS/cards.lock" | head -1)
cache=${DIRAC_GATE_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/dirac-gate}
mkdir -p "$cache" && find "$cache" -mindepth 1 -maxdepth 1 -mtime +3 -exec rm -rf {} + 2>/dev/null
ckey() { echo "$cache/$1-${corpus:-nocorpus}-c$INSTR_CORE-p$synth_pairs"; }
# cache_get <commit> <name>: the commit's stored value; fails when there is none.
cache_get() { local f; f=$(ckey "$1")/$2; [ -f "$f" ] && cat "$f"; }
# cache_put <commit> <name> <value>: stores a value measured on that commit, never on a dirty tree
# or without a corpus digest, since neither names what was measured.
cache_put() {
	[ -z "$dirty" ] && [ -n "$corpus" ] || return 0
	local d; d=$(ckey "$1"); mkdir -p "$d" && printf '%s\n' "$3" >"$d/$2.tmp" && mv "$d/$2.tmp" "$d/$2"
}
# The base side, built only when the cache misses.
base_wt() {
	[ -d "$S/base" ] && return 0
	git -C "$top" worktree add -q --detach "$S/base" "$mb" && (cd "$S/base" && make dirac-tables >/dev/null 2>&1)
}
base_test() { [ -x "$S/base.test" ] || { base_wt && (cd "$S/base" && go test -c -o "$S/base.test" ./dirac/); }; }
base_kfuzz() { [ -x "$S/kfuzz.base" ] || { base_wt && (cd "$S/base" && go build -o "$S/kfuzz.base" ./dirac/oracle/cmd/kfuzz); }; }
# base_value <name> <measure...>: the merge base's value from the cache, else measured (the measure
# builds what it needs) and stored. Exit 2 when the base cannot be built.
base_value() {
	local name=$1 v; shift
	if v=$(cache_get "$mb" "$name"); then printf '%s\n' "$v"; return 0; fi
	v=$("$@") || { echo "dirac gate: base build failed (infrastructure)" >&2; return 2; }
	[ -n "$v" ] && cache_put "$mb" "$name" "$v" # an empty measure (a crashed bench) is not stored
	printf '%s\n' "$v"
}

if [ -n "$dirac_changed" ]; then
	[ -n "$have_base" ] || echo "the merge base has no dirac/: base rows below are notes"
	go test -c -o "$S/head.test" ./dirac/ || { bad "dirac test binary does not build"; exit 1; }

	echo "== 2 State layout"
	size() { (cd "$2" && "$1" -test.run '^TestStateSize$' -test.v 2>&1 | sed -nE 's/.*State size: ([0-9]+) bytes.*/\1/p'); }
	base_size() { base_test && size "$S/base.test" "$S/base/dirac"; }
	sh=$(size "$S/head.test" "$top/dirac")
	[ -n "$sh" ] && cache_put "$head" state "$sh"
	barsrc=$top/dirac/kernel_test.go
	if [ -n "$have_base" ]; then
		barsrc=$S/bar_kernel_test.go
		git show "$mb:dirac/kernel_test.go" >"$barsrc" || exit 2
		bs=$(base_value state base_size) || exit 2
		echo "State: base $bs B, head ${sh:-?} B"
	else
		echo "State: head ${sh:-?} B"
	fi
	bar=$(sed -nE 's/^const maxStateBytes = ([0-9]+) \* 1024.*/\1/p' "$barsrc")
	bar=$((${bar:-0} * 1024))
	if [ -z "$sh" ] || [ "$bar" = 0 ]; then bad "could not read TestStateSize or maxStateBytes"
	elif [ "$sh" -gt "$bar" ]; then bad "State is $sh bytes, over the $bar B bar: crossing it is the maintainers' call"
	else echo "under the $bar B bar"; fi

	echo "== 3 allocs and speed (BenchmarkTransitionFDN)"
	bench() { (cd "$2" && taskset -c "$3" env GOMAXPROCS=1 "$1" -test.run XXX -test.bench 'BenchmarkTransitionFDN$' -test.benchtime 2s -test.benchmem 2>&1 | grep '^BenchmarkTransitionFDN'); }
	base_bench() { base_test && bench "$S/base.test" "$S/base/dirac" "$INSTR_CORE"; }
	bh=$(bench "$S/head.test" "$top/dirac" "$INSTR_CORE")
	ah=$(awk '{print $(NF-1)}' <<<"$bh")
	echo "head: $bh"
	[ "${ah:-x}" = 0 ] || bad "BenchmarkTransitionFDN allocates (${ah:-?} allocs/op)"
	[ -n "$bh" ] && cache_put "$head" bench "$bh"
	if [ -n "$have_base" ]; then
		bb=$(base_value bench base_bench) || exit 2
		r=$(awk -v h="$(awk '{print $3}' <<<"$bh")" -v b="$(awk '{print $3}' <<<"$bb")" 'BEGIN{printf "%.4f", h/b}')
		echo "head/base wall time $r"
		awk -v m="$r" -v x="$PERF_MAX" 'BEGIN{exit !(m > x)}' && echo "dirac gate: WARN (advisory) -- wall time ${r}x the base"
	fi
	echo "== 6 inlining"
	inl=$(go build -gcflags=-m=2 ./dirac 2>&1)
	hot='\(\*State\)\.(typesAt|basePT)\b|\(\*?slotMask\)\.|slotMask\.[a-zA-Z]+\b|slotBit\b'
	lost=$(grep -E "cannot inline ($hot)" <<<"$inl" | sed -E 's/^[^ ]+ //' | sort -u)
	seen=$(grep -cE 'can inline \(\*State\)\.(typesAt|basePT)\b' <<<"$inl")
	if [ -n "$lost" ]; then bad "a hot accessor stopped inlining: $lost"
	elif [ "$seen" -lt 2 ]; then bad "could not find (*State).typesAt and (*State).basePT in the -m=2 output"
	else echo ok; fi

	echo "== 7 escapes"
	# A local whose address reaches a heap-leaking call allocates on every visit: with GOGC off that
	# is unbounded growth during training.
	heap=$(grep -E '^[^ ]+\.go:[0-9]+:[0-9]+: moved to heap: ' <<<"$inl" | sort -u)
	if [ -n "$heap" ]; then bad "dirac locals moved to the heap: $heap"; else echo ok; fi

	echo "== 8 tests"
	out=$(cd "$top/dirac" && "$S/head.test" -test.timeout 2m 2>&1) && echo ok || bad "./dirac tests failed: $(tail -20 <<<"$out")"
	out=$(go test -timeout 2m ./dirac/crcoverage ./internal/archtest 2>&1) || bad "./dirac/crcoverage or ./internal/archtest failed: $(tail -20 <<<"$out")"
fi

# The whole refused list, the committed repro tables and the pinned games, whatever changed: a gorge
# fix leaves a refusal stale, moves a fixed repro off lockstep, or stops a pinned game reaching its
# condition as easily as a dirac one, and each otherwise first fails in the hourly train. A
# data-driven pin is a document under dirac/oracle/testdata/pins and plays as a TestPinManifest
# subtest, so a new one is gated with no edit here. The pins a Go test still drives are named by
# their files (<test>.<n>.json, a subtest's "/" written "__"). A skip is a pool or table missing on
# this machine, not a pass.
# TestChoosesHaveNoOther rides along: a choose kind posed outside the shared list passes every package the
# landing gate picks by diff and first failed in the hourly train (2026-11-09).
[ -n "$dirac_changed" ] || echo "== 8 tests (refused entries, repro tables, pinned games)"
pins=$(git ls-tree --name-only HEAD dirac/oracle/testdata/pins/ | sed -E 's|.*/||; s/__.*//; s/\.[0-9]+\.json$//' | sort -u | paste -sd'|')
[ -n "$pins" ] || bad "no pinned games under dirac/oracle/testdata/pins"
out=$(go test -v -timeout 5m -run "^TestRefusedEntriesStillDiverge\$|^TestChoosesHaveNoOther\$|^TestStd[A-Za-z]*ReprosLockstep|^TestPinManifest\$|^TestPinManifestCoversOldPins\$|^($pins)\$" ./dirac/oracle 2>&1) || bad "a refused entry no longer diverges, a committed repro left lockstep, or a pinned game no longer reaches its condition: $(named=$(grep -E -- '--- FAIL|divergen|never reaches' <<<"$out" | head -20); if [ -n "$named" ]; then echo "$named"; else echo "(no test failed by name: the test binary died or did not build)"; grep -m1 -A14 -E '^(panic:|fatal error:|# |FAIL[[:space:]])' <<<"$out"; tail -6 <<<"$out"; fi)"
skip=$(grep -F -- '--- SKIP' <<<"$out")
[ -z "$skip" ] || bad "a refused entry, repro or pinned game was not played, since its pool or tables are not on this machine (set GORGE_POOLS, make dirac-tables): $skip"

echo "== 4 lockstep, absolute"
lockstep() {
	"$S/kfuzz" -kernel "$1" -first-pair 0 -pairs 1500 -seed 1 -keep-going -surface "$2" -mulligans 1 -shrink 0 -workers "${GATE_KFUZZ_WORKERS:-4}" -absolute -v >"$S/lockstep-$1-$2.log" 2>&1
	echo $? >"$S/lockstep-$1-$2.rc"
}
# The four replays run two at a time (two surfaces of one set), reported in order below.
for set in fdn blb; do
	lockstep "$set" autopay &
	lockstep "$set" full &
	wait
done
for set in fdn blb; do
	for surface in autopay full; do
		log=$S/lockstep-$set-$surface.log
		rc=$(cat "$S/lockstep-$set-$surface.rc" 2>/dev/null || echo 2)
		grep "^$set:" "$log"
		if [ "$rc" -ne 0 ]; then
			# kfuzz -v prints a `pair` line per skip too; only the lines of a game that fails -absolute
			# (divergence, hang, unmodelled stall, error) name the pair to replay.
			failing='^pair [0-9]+ .*(seed [0-9]+\): (hang: no result after |error: )| unmodelled stall after |\) after [0-9]+ decisions:$)'
			grep -E "$failing|^ *(divergence at decision|options:|implicates:)" "$log" | head -6
			pair=$(grep -E "$failing" "$log" | sed -nE 's/^pair ([0-9]+) .*/\1/p;q')
			echo "  repro:   go run ./dirac/oracle/cmd/kfuzz -kernel $set -surface $surface -first-pair ${pair:-<pair>} -pairs 1 -seed 1 -mulligans 1 -absolute -v"
			echo "  Your change altered behaviour on a supported card. In this change, do one of:"
			echo "    1. fix dirac (dirac/*.go, dirac/kgen), or"
			echo "    2. refuse by name: go run ./dirac/cmd/kgen -refuse \"<card>\" -why \"...\" -repro $set:$surface:${pair:-<pair>}"
			bad "absolute lockstep, $set, $surface surface"
		fi
	done
done

if [ -n "$dirac_changed" ]; then
echo "== mp: vet, layout pin and lockstep, 4 seats and the 2-seat slices"
mp_t0=$SECONDS
if make --no-print-directory dirac-mp-check BIN_DIR="$S" >"$S/mp.log" 2>&1; then
	grep -E '^(vanilla|fdn|blb):' "$S/mp.log" | grep -v ' kernel capacity stalls '
	echo "mp: ok ($((SECONDS - mp_t0)) s)"
else
	grep -E '^(kfuzz-mp:|vanilla:|DIV |absolute:)|^(fdn|blb): .* clean' "$S/mp.log" | head -12
	tail -3 "$S/mp.log"
	bad "make dirac-mp-check failed ($((SECONDS - mp_t0)) s)"
fi
fi

if [ -n "$dirac_changed" ]; then
	echo "== 5 instructions (perf stat instructions:u, GOGC=off, (N500k-N100k)/400000; core $INSTR_CORE)"
	# perf stat exits with the bench's status: a bench that panics part-way prints nothing, never a
	# truncated count.
	instr() {
		local out rc
		out=$(cd "$2" && GOGC=off GOMEMLIMIT=3GiB GOMAXPROCS=1 taskset -c "$INSTR_CORE" perf stat -x, -e instructions:u "$1" -test.run XXX -test.bench "^$3\$" -test.benchtime "${4}x" 2>&1 >/dev/null)
		rc=$?
		[ "$rc" -eq 0 ] || return 0
		grep -qE '^(panic:|fatal error:|--- FAIL|FAIL)' <<<"$out" && return 0
		awk -F, '$3 ~ /^instructions/ && $1 ~ /^[0-9]+$/ {print $1}' <<<"$out"
	}
	per_op() {
		local a b
		a=$(instr "$1" "$2" "$3" 100000)
		b=$(instr "$1" "$2" "$3" 500000)
		[ -n "$a" ] && [ -n "$b" ] && echo $(((b - a) / 400000))
	}
	over() { awk -v h="$1" -v b="$2" -v x="$3" 'BEGIN{exit !(h / b > x)}'; }
	if ! command -v perf >/dev/null 2>&1 || [ -z "$(perf stat -x, -e instructions:u true 2>&1 | awk -F, '$1 ~ /^[0-9]+$/')" ]; then
		bad "perf stat unavailable (instructions:u not counted): this step cannot pass without it"
	else
		base_instr() { base_test && per_op "$S/base.test" "$S/base/dirac" "$1"; }
		for row in BenchmarkTransitionFDN:$INSTR_PB_FDN BenchmarkTransitionBLB:$INSTR_PB_BLB BenchmarkTransitionFDNPool:; do
			bn=${row%%:*} pb=${row#*:}
			ih=$(per_op "$S/head.test" "$top/dirac" "$bn")
			if [ -z "$ih" ]; then bad "$bn: bench crashed or missing"; continue; fi
			cache_put "$head" "instr-$bn" "$ih"
			im=
			if [ -n "$have_base" ]; then im=$(base_value "instr-$bn" base_instr "$bn") || exit 2; fi
			echo "$bn: head $ih instr/op, merge base ${im:-none} (max ${INSTR_MB_MAX}x), program base ${pb:-none} (max ${INSTR_PB_MAX}x)"
			if [ -n "$im" ] && over "$ih" "$im" "$INSTR_MB_MAX"; then bad "$bn: head $ih is over ${INSTR_MB_MAX}x the merge base's $im instructions"; fi
			if [ -n "$pb" ] && over "$ih" "$pb" "$INSTR_PB_MAX"; then bad "$bn: head $ih is over ${INSTR_PB_MAX}x the program base's $pb instructions"; fi
		done
	fi

fi

echo "== 9 synthetic pools, against the merge base"
# synth_divergent prints one line per divergent pair, then an end marker, so a pool with no
# divergent pair still caches a value.
synth_divergent() {
	"$1" -synth "$2" -first-pair 0 -pairs "$synth_pairs" -seed 1 -keep-going -shrink 0 -workers "${GATE_KFUZZ_WORKERS:-4}" -v 2>&1 |
		sed -nE 's/^pair ([0-9]+) \((.*), seed [0-9]+\) after .*/\1 \2/p' | sort -u
	echo '#end'
}
base_synth() { base_kfuzz && synth_divergent "$S/kfuzz.base" "$1"; }
if [ -z "$have_base" ]; then
	echo "the merge base has no dirac/: nothing to compare"
else
	# The head's pools replay concurrently while the base side is read from the cache (or measured).
	for pool in $synth_pools; do synth_divergent "$S/kfuzz" "$pool" >"$S/synth-head-$pool" & done
	for pool in $synth_pools; do base_value "synth-$pool" base_synth "$pool" >"$S/synth-base-$pool" || exit 2; done
	wait
	for pool in $synth_pools; do
		grep -qx '#end' "$S/synth-head-$pool" || { bad "$pool: the head's synthetic replay did not finish"; continue; }
		cache_put "$head" "synth-$pool" "$(cat "$S/synth-head-$pool")"
		onbase=$(grep -v '^#end$' "$S/synth-base-$pool")
		onhead=$(grep -v '^#end$' "$S/synth-head-$pool")
		fresh=$(comm -13 <(printf '%s\n' "$onbase") <(printf '%s\n' "$onhead") | grep -v '^$')
		echo "$pool: $(grep -c . <<<"$onbase") divergent on the merge base, $(grep -c . <<<"$onhead") on the head, $(grep -c . <<<"$fresh") new"
		if [ -n "$fresh" ]; then
			sed 's/^/  /' <<<"$fresh"
			echo "  repro:   go run ./dirac/oracle/cmd/kfuzz -synth $pool -first-pair <pair> -pairs 1 -seed 1 -shrink 0 -v"
			bad "newly divergent pairs on $pool"
		fi
	done
fi

[ "$fail" = 0 ] && echo "dirac gate: ok" && exit 0
exit 1
