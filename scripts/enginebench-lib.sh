# enginebench-lib.sh: shared settings for the enginebench scripts (sourced, not run).
#
# Every knob is an environment variable with a default:
#   BENCH_DIR   binaries, source snapshots, results, profiles
#               (default ${TMPDIR:-/tmp}/gorge-enginebench: rebuildable scratch)
#   DECKS       workload deck directory (default: the committed testdata/decks)
#   CARDS       corpus directory (default: the repo's .cards)
#   HEAVY_LOCK  optional flock file serialising heavy jobs; unset (the
#               default) takes no lock. Point it at a shared lock file when
#               several heavy jobs share one box.
#   MEM_MAX     systemd scope MemoryMax (default 24G)
#   BENCH_GOMEMLIMIT  GOMEMLIMIT inside the scope (default 16GiB)
#   NO_SCOPE    set to run without a systemd scope
#   BUDGET      wall seconds for ONE script's whole measured phase -- every row
#               and rep together, builds excluded (default 60). Each run's -secs is derived from it (budget_secs),
#               and a run that would start past the deadline is skipped.
#   LOAD_S      per-run startup estimate (corpus load) budget_secs reserves
#               (default 3)

REPO=$(git rev-parse --show-toplevel)
BENCH_DIR=${BENCH_DIR:-${TMPDIR:-/tmp}/gorge-enginebench}
DECKS=${DECKS:-$REPO/cmd/enginebench/testdata/decks}
CARDS=${CARDS:-$REPO/.cards}
CARDS=$(cd "$CARDS" && pwd -P)
mkdir -p "$BENCH_DIR"

# heavy CMD...: run CMD capped in a systemd user scope when one is available
# (MemoryMax=MEM_MAX, GOMEMLIMIT=BENCH_GOMEMLIMIT) and, only when HEAVY_LOCK
# is set, serialised on that lock (flock -o, so the lock fd is not inherited
# by anything CMD leaves running).
heavy() {
	local -a run=(env GOMEMLIMIT="${BENCH_GOMEMLIMIT:-16GiB}" "$@")
	if [ -z "${NO_SCOPE:-}" ] && command -v systemd-run >/dev/null 2>&1; then
		run=(systemd-run --user --scope -q -p MemoryMax="${MEM_MAX:-24G}" "${run[@]}")
	fi
	if [ -n "${HEAVY_LOCK:-}" ]; then
		flock -o "$HEAVY_LOCK" "${run[@]}"
	else
		"${run[@]}"
	fi
}

BUDGET=${BUDGET:-60}
LOAD_S=${LOAD_S:-3}

# budget_secs RUNS: the -secs each of RUNS runs gets so all of them, plus
# LOAD_S of startup apiece, fit in BUDGET (at least 1 s each).
budget_secs() {
	awk -v b="$BUDGET" -v l="$LOAD_S" -v n="$1" 'BEGIN { s = (b - n * l) / n; if (s < 1) s = 1; printf "%.1f", s }'
}

# budget_start: start the BUDGET clock (call after the builds).
budget_start() { BUDGET_DEADLINE=$(($(date +%s) + BUDGET)); }

# budget_left: true while the BUDGET clock has time left; otherwise it says so.
budget_left() {
	[ "$(date +%s)" -lt "${BUDGET_DEADLINE:?budget_start not called}" ] && return 0
	echo "BUDGET ${BUDGET}s spent: skipping the remaining runs" >&2
	return 1
}

# workload_args: the flags every run passes explicitly, so a binary built at an
# older revision (whose defaults point elsewhere) plays the same workload.
workload_args() {
	echo -decks "$DECKS" -burn "$DECKS/burn.json" -cards "$CARDS"
}

# The default row set (the 2026-10-05 profiling loop's rows): ';'-separated
# enginebench argument strings. SECS overrides each row's -secs.
DEFAULT_ROWS="-row random -pair A;-row random -pair B;-row bot -pair A;-row sampler"
