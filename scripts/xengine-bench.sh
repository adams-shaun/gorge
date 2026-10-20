#!/usr/bin/env bash
# xengine-bench.sh [ENGINE...]: whole-game throughput of gorge against other
# Magic rules engines on one box, on the same decks, one pinned core per run.
#
# ENGINE is any of gorge xmage forge dirty-dirac mtg-kernel (default: every engine that
# is set up; gorge always is). Each engine plays FDN pair A
# (FDN_top_04956_UG vs FDN_top_20626_WG) and pair B (FDN_top_07961_WR vs
# FDN_top_02581_UR) from cmd/enginebench/testdata/decks, under each policy it
# offers (see docs/site/performance/methodology.md for what each policy is).
#
# Every run is one single-threaded process pinned to one core with taskset
# (GOMAXPROCS=1 for Go). REPS repetitions of a row run at the same time, one
# per core in CORES; rows and pairs run one after another. Each run appends one
# JSON line to $OUT/runs.jsonl (engine, row, pair, rep, core, load before and
# after, peak RSS, and the engine's own counts), and the summary table is
# printed at the end (scripts/xengine-bench-summarize.py).
#
# Knobs (environment):
#   OUT      results directory (default ./xengine-out)
#   CORES    one core per concurrent rep (default "24 26 28"; REPS = their count)
#   SECS     measured seconds per run (default 30)
#   WARM     JVM warm-up seconds before measuring, XMage and Forge (default 30)
#   PAIRS    deck pairs (default "A B"; "burn" is the Burn mirror, which dirty-dirac cannot play)
#   JAVA     java binary for XMage and Forge (default: java on PATH)
#   JAVA_HEAP  -Xmx for both JVMs (default 2g)
#   gorge:   nothing; it builds cmd/enginebench from this checkout (CARDS=.cards)
#   xmage:   XMAGE_TESTS   a built XMage Mage.Tests directory (target/classes,
#                          target/test-classes, config/, db/cards.h2.mv.db)
#            XMAGE_CP      file holding Mage.Tests' test classpath
#                          (mvn dependency:build-classpath -Dmdep.includeScope=test)
#            XMAGE_BENCH   XEngineBench.java (gorge-compliance
#                          oracle/xmageoracle/bench/org/mage/test/bench/)
#   forge:   FORGE_CP      file holding the forge-core/game/ai classpath
#            FORGE_RES     Forge's forge-gui/res directory
#            FORGE_SRC     the forge-oracle driver sources (src/main/java with
#                          forge.oracle.GameReplay)
#   dirty-dirac: KPLAY     a kplay binary (private module; skipped when unset)
#   mtg-kernel: MTGKERNEL_BENCH  bench_kernel built with the xengine mode
#            (cmd/enginebench/scripts/xengine/mtgkernel/); MK_PAIRS (default A),
#            MK_WARM warm-up CPU-seconds (default 5), MK_ROW the row label
#            (default random; label a second build's rows differently)
set -euo pipefail

REPO=$(cd "$(dirname "$0")/.." && pwd)
OUT=$(realpath -m "${OUT:-./xengine-out}")
read -r -a CORE_LIST <<< "${CORES:-24 26 28}"
SECS=${SECS:-30}
WARM=${WARM:-30}
read -r -a PAIR_LIST <<< "${PAIRS:-A B}"
JAVA=${JAVA:-java}
JAVA_HEAP=${JAVA_HEAP:-2g}
CARDS=$(realpath "${CARDS:-$REPO/.cards}")
DECKS=$REPO/cmd/enginebench/testdata/decks
mkdir -p "$OUT/work"
RUNS=$OUT/runs.jsonl

die() { echo "xengine-bench: $*" >&2; exit 2; }
load1() { cut -d' ' -f1 /proc/loadavg; }
deck_a() { case $1 in A) echo FDN_top_04956_UG ;; B) echo FDN_top_07961_WR ;; burn) echo burn ;; *) die "pair $1: want A, B or burn" ;; esac; }
deck_b() { case $1 in A) echo FDN_top_20626_WG ;; B) echo FDN_top_02581_UR ;; burn) echo burn ;; esac; }
# Every driver but enginebench reads .dck lists from DK: the four FDN decks,
# and the Burn mirror's burn.json written out as burn.dck ("N Name" lines).
DK=$OUT/work/decks
mkdir -p "$DK"
for f in "$DECKS"/*.dck; do ln -sf "$f" "$DK/"; done
python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); open(sys.argv[2],"w").write("NAME:Burn\n"+"".join("%d %s\n" % (c["count"], c["name"]) for c in d["cards"]))' \
	"$DECKS/burn.json" "$DK/burn.dck"
# XMage resolves set-less lines only through its MTGO-text importer (.txt).
sed 1d "$DK/burn.dck" > "$DK/burn.txt"

# corestat CORE: that core's /proc/stat line (busy and idle jiffies).
corestat() { awk -v c="cpu$1" '$1 == c' /proc/stat; }

# record ENGINE ROW PAIR REP CORE LOAD0 TIMEFILE RESULTFILE KIND: append one run
# line. The core's /proc/stat delta since the run started ($w.stat0) says how
# busy the core was beyond the run itself: another process on it is recorded
# as other_busy, so a contended rep can be spotted and rerun.
record() {
	local w=${7%.time}
	corestat "$5" > "$w.stat1"
	python3 -I "$REPO/scripts/xengine-bench-summarize.py" --record "$@" "$(load1)" >> "$RUNS"
}

# pinned CORE TIMEFILE CMD...: run CMD on one core, its peak RSS and CPU in TIMEFILE.
pinned() {
	local core=$1 tf=$2
	shift 2
	/usr/bin/time -o "$tf" -f '{"maxrss_kb":%M,"user_s":%U,"sys_s":%S,"wall_s":%e}' taskset -c "$core" "$@"
}

# batch NAME FN ARGS...: run FN ARGS... REP CORE once per core, concurrently.
batch() {
	local name=$1
	shift
	local pids=() rep=0
	echo "== $name (load1 $(load1), ${#CORE_LIST[@]} reps on cores ${CORE_LIST[*]})" >&2
	for core in "${CORE_LIST[@]}"; do
		"$@" "$rep" "$core" & pids+=($!)
		rep=$((rep + 1))
	done
	local fail=0
	for p in "${pids[@]}"; do wait "$p" || fail=1; done
	[ "$fail" = 0 ] || echo "xengine-bench: a run of $name failed; see $OUT/work" >&2
}

# ---- gorge (cmd/enginebench) -------------------------------------------------
gorge_bin=$OUT/work/enginebench
gorge_setup() {
	(cd "$REPO" && go build -ldflags "-X main.commit=$(git rev-parse HEAD)" -o "$gorge_bin" ./cmd/enginebench)
}
gorge_run() { # ROW PAIR FLAGS REP CORE
	local row=$1 pair=$2 flags=$3 rep=$4 core=$5
	local w=$OUT/work/gorge-$row-$pair-$rep l0
	l0=$(load1); corestat "$core" > "$w.stat0"
	# shellcheck disable=SC2086
	(cd "$REPO" && pinned "$core" "$w.time" env GOMAXPROCS=1 "$gorge_bin" -row "${row%%-*}" $flags -pair "$pair" \
		-secs "$SECS" -seed $((1 + 1000003 * rep)) -cards "$CARDS" -decks "$DECKS" -out "$w.json" 2>"$w.log")
	record gorge "$row" "$pair" "$rep" "$core" "$l0" "$w.time" "$w.json" enginebench
}
gorge_all() {
	gorge_setup
	for pair in "${PAIR_LIST[@]}"; do
		batch "gorge random-autopay $pair" gorge_run random-autopay "$pair" "-autopay"
		batch "gorge random $pair" gorge_run random "$pair" ""
		batch "gorge bot-autopay $pair" gorge_run bot-autopay "$pair" "-autopay"
		batch "gorge bot $pair" gorge_run bot "$pair" ""
	done
}

# ---- XMage (gorge-compliance XEngineBench) -----------------------------------
xmage_ready() { [ -n "${XMAGE_TESTS:-}" ] && [ -n "${XMAGE_CP:-}" ] && [ -n "${XMAGE_BENCH:-}" ]; }
xmage_setup() {
	[ -d "$XMAGE_TESTS/target/test-classes" ] || die "XMAGE_TESTS=$XMAGE_TESTS has no target/test-classes"
	XCP="$OUT/work/xmage-classes:$XMAGE_TESTS/target/test-classes:$XMAGE_TESTS/target/classes:$(cat "$XMAGE_CP")"
	mkdir -p "$OUT/work/xmage-classes"
	"${JAVA%java}javac" -nowarn -d "$OUT/work/xmage-classes" -cp "$XCP" "$XMAGE_BENCH"
}
xmage_run() { # POLICY PAIR REP CORE
	local pol=$1 pair=$2 rep=$3 core=$4
	local w=$OUT/work/xmage-$pol-$pair-$rep l0 cwd xext=dck
	[ "$pair" = burn ] && xext=txt
	# A private cwd per JVM: XMage's H2 card DB (db/cards.h2) cannot be shared
	# between JVMs, so each run gets a copy and links to the rest of Mage.Tests.
	cwd=$w.cwd
	rm -rf "$cwd" && mkdir -p "$cwd/db"
	for e in "$XMAGE_TESTS"/*; do [ "$(basename "$e")" = db ] || ln -s "$e" "$cwd/"; done
	cp "$XMAGE_TESTS/db/cards.h2.mv.db" "$cwd/db/"
	l0=$(load1); corestat "$core" > "$w.stat0"
	(cd "$cwd" && pinned "$core" "$w.time" "$JAVA" -Xmx"$JAVA_HEAP" -Dlog4j.configuration=file:/dev/null -cp "$XCP" \
		org.mage.test.bench.XEngineBench "$pol" "$DK/$(deck_a "$pair").$xext" "$DK/$(deck_b "$pair").$xext" \
		"$WARM" "$SECS" $((1 + 1000003 * rep)) 100 1 >"$w.jsonl" 2>"$w.log")
	rm -rf "$cwd"
	record xmage "$pol" "$pair" "$rep" "$core" "$l0" "$w.time" "$w.jsonl" xmage
}
xmage_all() {
	xmage_setup
	for pair in "${PAIR_LIST[@]}"; do
		for pol in random simple mad; do
			batch "xmage $pol $pair" xmage_run "$pol" "$pair"
		done
	done
}

# ---- Forge (forge-oracle GameReplay: Forge's AI on both seats) ---------------
forge_ready() { [ -n "${FORGE_CP:-}" ] && [ -n "${FORGE_RES:-}" ] && [ -n "${FORGE_SRC:-}" ]; }
forge_setup() {
	FCP="$OUT/work/forge-classes:$(cat "$FORGE_CP")"
	mkdir -p "$OUT/work/forge-classes"
	# shellcheck disable=SC2046
	"${JAVA%java}javac" --release 17 -encoding UTF-8 -nowarn -cp "$(cat "$FORGE_CP")" -d "$OUT/work/forge-classes" \
		$(find "$FORGE_SRC" -name '*.java' | sort)
}
# kill_java_below PID: TERM every java process in PID's process tree.
kill_java_below() {
	local k
	for k in $(cat /proc/"$1"/task/*/children 2>/dev/null); do
		if [ "$(cat /proc/"$k"/comm 2>/dev/null)" = java ]; then kill "$k" 2>/dev/null || true; fi
		kill_java_below "$k"
	done
}
forge_run() { # PAIR REP CORE
	local pair=$1 rep=$2 core=$3
	local w=$OUT/work/forge-ai-$pair-$rep l0 pid
	python3 -I "$REPO/scripts/xengine-bench-summarize.py" --forge-games "$DK" "$(deck_a "$pair")" "$(deck_b "$pair")" \
		$((1 + 1000003 * rep)) 2000 > "$w.in.jsonl"
	mkdir -p "$w.tmp"
	l0=$(load1); corestat "$core" > "$w.stat0"
	# GameReplay plays the list to the end; stop it once WARM+SECS of game
	# time is written (rows are flushed per game). Warm-up rows are dropped
	# when the run is recorded.
	(cd "$w.tmp" && pinned "$core" "$w.time" env FORGE_RES="$FORGE_RES" "$JAVA" -Djava.awt.headless=true -Xmx"$JAVA_HEAP" \
		-Dtinylog.writer.level=warn -Dtinylog.level=warn \
		-Djava.io.tmpdir="$w.tmp" -cp "$FCP" forge.oracle.GameReplay "$w.in.jsonl" "$w.jsonl" >"$w.log" 2>&1) & pid=$!
	local target=$(( (WARM + SECS) * 1000 ))
	while kill -0 "$pid" 2>/dev/null; do
		sleep 2
		if [ -s "$w.jsonl" ] && [ "$(python3 -I -c 'import json,sys; print(sum(json.loads(l).get("ms",0) for l in open(sys.argv[1]) if l.strip()))' "$w.jsonl" 2>/dev/null || echo 0)" -ge "$target" ]; then
			# Signal the JVM (a descendant of $pid, under time), not time,
			# so time still writes its report.
			kill_java_below "$pid"
			break
		fi
	done
	wait "$pid" || true
	rm -rf "$w.tmp"
	record forge ai "$pair" "$rep" "$core" "$l0" "$w.time" "$w.jsonl" "forge:$WARM:$SECS"
}
forge_all() {
	forge_setup
	for pair in "${PAIR_LIST[@]}"; do
		batch "forge ai $pair" forge_run "$pair"
	done
}

# ---- dirty-dirac (the private kernel's kplay) --------------------------------
kplay_run() { # POLICY PAIR REP CORE
	local pol=$1 pair=$2 rep=$3 core=$4
	local w=$OUT/work/dirty-dirac-$pol-$pair-$rep l0
	python3 -I "$REPO/scripts/xengine-bench-summarize.py" --kplay-decks "$DK" "$(deck_a "$pair")" "$(deck_b "$pair")" \
		"$w.decks.jsonl" "$w.pairs.tsv" $((1 + 1000003 * rep))
	l0=$(load1); corestat "$core" > "$w.stat0"
	pinned "$core" "$w.time" env GOMAXPROCS=1 "$KPLAY" -cards "$CARDS" -deckfile "$w.decks.jsonl" -pairs-file "$w.pairs.tsv" \
		-pairs 0 -a "$pol" -b "$pol" -wall "${SECS}s" -out "$w.jsonl" >"$w.log" 2>&1
	record dirty-dirac "$pol" "$pair" "$rep" "$core" "$l0" "$w.time" "$w.jsonl.summary.json" kplay
}
kplay_all() {
	for pair in "${PAIR_LIST[@]}"; do
		batch "dirty-dirac random $pair" kplay_run random "$pair"
		batch "dirty-dirac hbot $pair" kplay_run hbot "$pair"
	done
}

# ---- mtg-kernel (jackmaiorino/mtg-kernel, with the xengine bench mode) -------
# MTGKERNEL_BENCH is mtg-kernel's bench_kernel example built with the
# --xengine-json mode from cmd/enginebench/scripts/xengine/mtgkernel/ and the
# limited-fdn-fixtures feature (methodology.md, "mtg-kernel"). It implements a
# few dozen FDN cards: it plays pair A and refuses a pair it cannot play, so
# the default pair list for it is A.
mtgkernel_run() { # PAIR REP CORE
	local pair=$1 rep=$2 core=$3
	local w=$OUT/work/mtg-kernel-${MK_ROW:-random}-$pair-$rep l0
	l0=$(load1); corestat "$core" > "$w.stat0"
	pinned "$core" "$w.time" "$MTGKERNEL_BENCH" --xengine-json "$DK/$(deck_a "$pair").dck" "$DK/$(deck_b "$pair").dck" \
		"${MK_WARM:-5}" "$SECS" $((1 + 1000003 * rep)) >"$w.json" 2>"$w.log" || true
	record mtg-kernel "${MK_ROW:-random}" "$pair" "$rep" "$core" "$l0" "$w.time" "$w.json" mtgkernel
}
mtgkernel_all() {
	local pairs
	read -r -a pairs <<< "${MK_PAIRS:-A}"
	for pair in "${pairs[@]}"; do
		batch "mtg-kernel ${MK_ROW:-random} $pair" mtgkernel_run "$pair"
	done
}

engines=("$@")
if [ ${#engines[@]} -eq 0 ]; then
	engines=(gorge)
	xmage_ready && engines+=(xmage)
	forge_ready && engines+=(forge)
	[ -n "${KPLAY:-}" ] && engines+=(dirty-dirac)
	[ -n "${MTGKERNEL_BENCH:-}" ] && engines+=(mtg-kernel)
fi
{
	echo "xengine-bench $(date -u +%FT%TZ) host=$(uname -n) cpu=\"$(sed -n 's/^model name\s*: //p' /proc/cpuinfo | head -1)\""
	echo "gorge $(git -C "$REPO" rev-parse HEAD) $(go version 2>/dev/null)"
	echo "java $("$JAVA" -version 2>&1 | head -1)"
	echo "engines=${engines[*]} cores=${CORE_LIST[*]} secs=$SECS warm=$WARM pairs=${PAIR_LIST[*]} heap=$JAVA_HEAP"
} | tee -a "$OUT/run.txt" >&2
for e in "${engines[@]}"; do
	case $e in
	gorge) gorge_all ;;
	xmage) xmage_ready || die "xmage: set XMAGE_TESTS, XMAGE_CP and XMAGE_BENCH"; xmage_all ;;
	forge) forge_ready || die "forge: set FORGE_CP, FORGE_RES and FORGE_SRC"; forge_all ;;
	dirty-dirac) [ -n "${KPLAY:-}" ] || die "dirty-dirac: set KPLAY"; kplay_all ;;
	mtg-kernel) [ -n "${MTGKERNEL_BENCH:-}" ] || die "mtg-kernel: set MTGKERNEL_BENCH"; mtgkernel_all ;;
	*) die "unknown engine $e (gorge xmage forge dirty-dirac mtg-kernel)" ;;
	esac
done
python3 -I "$REPO/scripts/xengine-bench-summarize.py" "$RUNS"
