#!/usr/bin/env bash
# dirac-gate.sh <base> -- what a change to dirac, or to anything dirac's oracle plays against,
# must keep true. Every comparison is with the merge base. A pipeline reads this file from <base>
# (`git show <base>:scripts/dirac-gate.sh | bash -s <base>`), so a branch cannot weaken the gate
# that judges it.
#
#   0. path filter: skipped when no changed file is in kfuzz's dependency closure and FORGE_REF
#      did not move; test files in that closure do not count.
#   1. protected: no edit to this file, to maxStateBytes or to the evidence constants.
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
#   8. tests: ./dirac, ./dirac/crcoverage, internal/archtest, and any refused entry the branch adds.
#
# Steps 1-3 and 5-8 run only when a non-test file under dirac/ changed. A merge base without dirac/
# has nothing to compare, so the base rows print a note. About 6 minutes on 4 cores with dirac/
# changed, 2 with only its dependencies changed.
set -uo pipefail
base=${1:?usage: dirac-gate.sh <base>}
INSTR_MB_MAX=${INSTR_MB_MAX:-1.005}
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
bad() { echo "dirac gate: FAIL -- $*"; fail=1; }

echo "== 0 path filter"
closure=$(go list -deps -f '{{if not .Standard}}{{.Dir}}{{end}}' ./dirac/oracle/cmd/kfuzz) || { echo "dirac gate: go list failed (infrastructure)"; exit 2; }
closure=$(sed "s|^$top/||" <<<"$closure" | sort -u)
in_closure=$(while IFS= read -r f; do
	case $f in *_test.go|'') continue ;; esac
	if grep -qxF "$(dirname "$f")" <<<"$closure"; then printf '%s\n' "$f"; fi
done <<<"$changed")
forge_ref_moved=
if grep -qx Makefile <<<"$changed" && git diff "$mb" HEAD -- Makefile | grep -qE '^[-+]FORGE_REF'; then forge_ref_moved=1; fi
if [ -z "$in_closure" ] && [ -z "$forge_ref_moved" ]; then
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

have_base=
if [ -n "$dirac_changed" ]; then
	echo "== 1 protected"
	prot=""
	if git cat-file -e "$mb:scripts/dirac-gate.sh" 2>/dev/null && grep -qx 'scripts/dirac-gate.sh' <<<"$changed"; then prot="scripts/dirac-gate.sh"; fi
	if git cat-file -e "$mb:dirac/kernel_test.go" 2>/dev/null; then
		git diff "$mb" HEAD -- dirac/kernel_test.go | grep -qE '^[-+].*maxStateBytes' && prot="$prot maxStateBytes"
		git diff "$mb" HEAD -- dirac/oracle/evidence.go | grep -qE '^[-+].*(Evidence[A-Z]|Synth[A-Z]|TailBucket)' && prot="$prot evidence constants"
		have_base=1
	else
		echo "the merge base has no dirac/: base rows below are notes"
	fi
	[ -z "$prot" ] && echo ok || bad "edits what only the maintainers change: $prot"

	go test -c -o "$S/head.test" ./dirac/ || { bad "dirac test binary does not build"; exit 1; }
	if [ -n "$have_base" ]; then
		git -C "$top" worktree add -q --detach "$S/base" "$mb" || exit 2
		(cd "$S/base" && make dirac-tables >/dev/null 2>&1 && go test -c -o "$S/base.test" ./dirac/) || { echo "dirac gate: base build failed (infrastructure)"; exit 2; }
	fi

	echo "== 2 State layout"
	size() { (cd "$2" && "$1" -test.run '^TestStateSize$' -test.v 2>&1 | sed -nE 's/.*State size: ([0-9]+) bytes.*/\1/p'); }
	sh=$(size "$S/head.test" "$top/dirac")
	barsrc=$top/dirac/kernel_test.go
	if [ -n "$have_base" ]; then
		barsrc=$S/base/dirac/kernel_test.go
		echo "State: base $(size "$S/base.test" "$S/base/dirac") B, head ${sh:-?} B"
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
	bh=$(bench "$S/head.test" "$top/dirac" "$INSTR_CORE")
	ah=$(awk '{print $(NF-1)}' <<<"$bh")
	echo "head: $bh"
	[ "${ah:-x}" = 0 ] || bad "BenchmarkTransitionFDN allocates (${ah:-?} allocs/op)"
	if [ -n "$have_base" ]; then
		bb=$(bench "$S/base.test" "$S/base/dirac" "$INSTR_CORE")
		r=$(awk -v h="$(awk '{print $3}' <<<"$bh")" -v b="$(awk '{print $3}' <<<"$bb")" 'BEGIN{printf "%.4f", h/b}')
		echo "head/base wall time $r"
		awk -v m="$r" -v x="$PERF_MAX" 'BEGIN{exit !(m > x)}' && echo "dirac gate: WARN (advisory) -- wall time ${r}x the base"
	fi
fi

echo "== 4 lockstep, absolute"
for set in fdn blb; do
	for surface in autopay full; do
		log=$S/lockstep-$set-$surface.log
		"$S/kfuzz" -kernel "$set" -first-pair 0 -pairs 1500 -seed 1 -keep-going -surface "$surface" -mulligans 1 -shrink 0 -workers "${GATE_KFUZZ_WORKERS:-4}" -absolute >"$log" 2>&1
		rc=$?
		grep "^$set:" "$log"
		if [ "$rc" -ne 0 ]; then
			grep -E '^ *(pair |options:|implicates:)' "$log" | head -6
			echo "  repro:   go run ./dirac/oracle/cmd/kfuzz -kernel $set -surface $surface -first-pair <pair> -pairs 1 -seed 1 -mulligans 1 -absolute -v"
			echo "  Your change altered behaviour on a supported card. In this change, do one of:"
			echo "    1. fix dirac (dirac/*.go, dirac/kgen), or"
			echo "    2. refuse by name: go run ./dirac/cmd/kgen -refuse \"<card>\" -why \"...\" -repro $set:$surface:<pair>"
			bad "absolute lockstep, $set, $surface surface"
		fi
	done
done

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
		for row in BenchmarkTransitionFDN:$INSTR_PB_FDN BenchmarkTransitionBLB:$INSTR_PB_BLB BenchmarkTransitionFDNPool:; do
			bn=${row%%:*} pb=${row#*:}
			ih=$(per_op "$S/head.test" "$top/dirac" "$bn")
			if [ -z "$ih" ]; then bad "$bn: bench crashed or missing"; continue; fi
			im=
			[ -n "$have_base" ] && im=$(per_op "$S/base.test" "$S/base/dirac" "$bn")
			echo "$bn: head $ih instr/op, merge base ${im:-none} (max ${INSTR_MB_MAX}x), program base ${pb:-none} (max ${INSTR_PB_MAX}x)"
			if [ -n "$im" ] && over "$ih" "$im" "$INSTR_MB_MAX"; then bad "$bn: head $ih is over ${INSTR_MB_MAX}x the merge base's $im instructions"; fi
			if [ -n "$pb" ] && over "$ih" "$pb" "$INSTR_PB_MAX"; then bad "$bn: head $ih is over ${INSTR_PB_MAX}x the program base's $pb instructions"; fi
		done
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
	out=$(KERNEL_REFUSED_BASE=$mb go test -timeout 2m -run '^TestRefusedEntriesStillDiverge$' ./dirac/oracle 2>&1) || bad "a refused entry no longer diverges: $(tail -20 <<<"$out")"
fi

[ "$fail" = 0 ] && echo "dirac gate: ok" && exit 0
exit 1
