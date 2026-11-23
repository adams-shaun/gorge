#!/usr/bin/env bash
# bench-tests.sh: per-package and per-test cost under the test budget (4 GiB RSS, 4 vCPU, 1 minute wall).
#
#   make bench-tests [PKGS='./p ./q'] [LANES=1] [SHARD=i/n] [BENCH_LOG_DIR=...]
#
# PKGS defaults to every package with test files (`go list`). Each package runs once in its own
# `systemd-run --user --scope -p MemoryMax=4G`, with GOMAXPROCS=4, GOFLAGS='-p=1 -trimpath', and
# `go test -count=1 -json -timeout 15m`. -count=1 is the point here: this measures cost, not whether the
# test cache can answer. The raw -json stream stays in $BENCH_LOG_DIR (default .ds4/scratch/bench-tests);
# stdout is TSV only:
#
#   kind	pkg	test	elapsed_s	wall_s	cpu_s	peak_rss_mb	tests	cached	reason
#   pkg	<pkg>	-	-	<wall>	<cpu>	<rss>	<tests>	<0|1>	-
#   test	<pkg>	<test>	<seconds>	-	-	-	-	-	-
#   over	<pkg>	<test>	<seconds>	-	-	-	-	-	test_over_60s
#   over	<pkg>	-	-	<wall>	<cpu>	<rss>	<tests>	<0|1>	rss_over_4g|timeout
#
# The pkg row is one package run; test rows are top-level tests (a parent includes its subtests); over rows
# name a test over 60 s, a package over 4 GiB, or a package timeout. cpu_s is the scope's cgroup cpu.stat
# usage_usec and peak_rss_mb its memory.peak. A skip whose message says the corpus or pools are missing is
# ENV, using scripts/test_targets.py's one ENV_SKIP rule.
#
# Exit: 0 measured, 1 a package failed, 2 usage, 3 an environment skip, 5 a package timed out or went over
# 4 GiB. A measurement that is merely slow still exits 0 and prints its over row.
set -uo pipefail

top=$(git rev-parse --show-toplevel) || { echo "bench-tests: not in a git worktree" >&2; exit 2; }
cd "$top" || exit 2
py=(python3 -I "$top/scripts/bench_tests.py")
header=$'kind\tpkg\ttest\telapsed_s\twall_s\tcpu_s\tpeak_rss_mb\ttests\tcached\treason'

# __run <pkg> <log-base>: the command inside the scope. It writes the raw -json stream, copies the cgroup
# probes next to it (the scope's cgroup disappears when this exits), and records wall/rc for the summarizer.
if [ "${1:-}" = __run ]; then
	pkg=${2:?__run needs a package}
	base=${3:?__run needs a log base}
	start=$(date +%s%N) || start=0
	GOMAXPROCS=4 GOFLAGS='-p=1 -trimpath' go test -count=1 -json -timeout 15m "$pkg" >"$base.json" 2>&1
	rc=$?
	end=$(date +%s%N) || end=$start

	# cgroup v2: one unified path, cpu.stat and memory.peak. v1: the cpuacct and memory mounts separately.
	cg=$(awk -F: '$1 == "0" { print $3; exit }' /proc/self/cgroup 2>/dev/null)
	if [ -n "$cg" ] && [ -d "/sys/fs/cgroup$cg" ]; then
		[ ! -f "/sys/fs/cgroup$cg/cpu.stat" ] || cp -f "/sys/fs/cgroup$cg/cpu.stat" "$base.cpu.stat" 2>/dev/null
		[ ! -f "/sys/fs/cgroup$cg/memory.peak" ] || cp -f "/sys/fs/cgroup$cg/memory.peak" "$base.memory.peak" 2>/dev/null
	else
		cpu_cg=$(awk -F: '$2 ~ /(^|,)cpuacct(,|$)/ { print $3; exit }' /proc/self/cgroup 2>/dev/null)
		if [ -n "$cpu_cg" ] && [ -r "/sys/fs/cgroup/cpuacct$cpu_cg/cpuacct.usage" ]; then
			read -r ns <"/sys/fs/cgroup/cpuacct$cpu_cg/cpuacct.usage" || ns=0
			[[ $ns =~ ^[0-9]+$ ]] && printf 'usage_usec %s\n' "$((ns / 1000))" >"$base.cpu.stat"
		fi
		mem_cg=$(awk -F: '$2 ~ /(^|,)memory(,|$)/ { print $3; exit }' /proc/self/cgroup 2>/dev/null)
		if [ -n "$mem_cg" ] && [ -r "/sys/fs/cgroup/memory$mem_cg/memory.max_usage_in_bytes" ]; then
			cp -f "/sys/fs/cgroup/memory$mem_cg/memory.max_usage_in_bytes" "$base.memory.peak" 2>/dev/null
		fi
	fi
	printf 'wall_ns=%s\nrc=%s\n' "$((end - start))" "$rc" >"$base.meta"
	exit "$rc"
fi

LANES=${LANES:-1}
case $LANES in '' | *[!0-9]* | 0) echo "bench-tests: LANES must be a positive integer (got '$LANES')" >&2; exit 2 ;; esac
BENCH_LOG_DIR=${BENCH_LOG_DIR:-.ds4/scratch/bench-tests}
[ -n "$BENCH_LOG_DIR" ] || { echo "bench-tests: BENCH_LOG_DIR is empty" >&2; exit 2; }
mkdir -p "$BENCH_LOG_DIR" || exit 2

# PKGS: verbatim package arguments, whitespace-separated. Default: every package with test files.
if [ -n "${PKGS:-}" ]; then
	mapfile -t pkgs < <(printf '%s\n' "$PKGS" | tr '[:space:]' '\n' | sed '/^$/d')
else
	if ! go list -e -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... 2>"$BENCH_LOG_DIR/go-list.err" >"$BENCH_LOG_DIR/go-list.out"; then
		echo "bench-tests: go list ./... failed; see $BENCH_LOG_DIR/go-list.err" >&2
		exit 2
	fi
	mapfile -t pkgs < <("${py[@]}" go-list <"$BENCH_LOG_DIR/go-list.out")
fi
mapfile -t pkgs < <(printf '%s\n' "${pkgs[@]}" | sed '/^$/d' | sort -u)
for pkg in "${pkgs[@]}"; do
	[[ $pkg =~ ^[A-Za-z0-9_./:-]+$ && $pkg != *..* ]] ||
		{ echo "bench-tests: refusing suspicious package argument '$pkg'" >&2; exit 2; }
done
if [ -n "${SHARD:-}" ]; then
	selected=$(printf '%s\n' "${pkgs[@]}" | "${py[@]}" shard --spec "$SHARD") || exit 2
	if [ -n "$selected" ]; then mapfile -t pkgs <<<"$selected"; else pkgs=(); fi
fi
printf '%s\n' "$header"

have_sd=0
if systemd-run --user --scope -q true >/dev/null 2>&1; then have_sd=1; else echo "bench-tests: note: no user systemd; running uncapped" >&2; fi
tmp=$(mktemp -d "${TMPDIR:-/tmp}/bench-tests.XXXXXX") || exit 2
mkdir -p "$tmp/rows" "$tmp/rc"
trap 'rm -rf "$tmp"' EXIT

# A flat package label becomes a path under the log dir; the import path already has the shards' directories.
log_base() {
	local rel=${1#./}
	case $rel in '' | .) rel=_root_ ;; esac
	printf '%s/%s' "$BENCH_LOG_DIR" "$rel"
}

run_pkg() {
	local pkg=$1 idx=$2 base wall_ns meta_rc rc pyrc
	base=$(log_base "$pkg")
	mkdir -p "$(dirname "$base")"
	rm -f "$base.json" "$base.cpu.stat" "$base.memory.peak" "$base.meta"
	if [ "$have_sd" -eq 1 ]; then
		systemd-run --user --scope -q -p MemoryMax=4G \
			env GOMAXPROCS=4 GOFLAGS='-p=1 -trimpath' \
			bash "$top/scripts/bench-tests.sh" __run "$pkg" "$base"
		rc=$?
	else
		bash "$top/scripts/bench-tests.sh" __run "$pkg" "$base"
		rc=$?
	fi
	wall_ns=0 meta_rc=$rc
	if [ -f "$base.meta" ]; then
		wall_ns=$(sed -n 's/^wall_ns=//p' "$base.meta")
		meta_rc=$(sed -n 's/^rc=//p' "$base.meta")
	fi
	"${py[@]}" summarize --pkg "$pkg" --json "$base.json" \
		--cpu-stat "$base.cpu.stat" --memory-peak "$base.memory.peak" \
		--wall-ns "${wall_ns:-0}" --rc "${meta_rc:-$rc}" --no-header >"$tmp/rows/$idx.tsv"
	pyrc=$?
	printf '%s\n' "$pyrc" >"$tmp/rc/$idx"
	[ "$pyrc" -eq 0 ] || echo "bench-tests: $pkg: see $base.json" >&2
}

idx=0
running=0
for pkg in "${pkgs[@]}"; do
	run_pkg "$pkg" "$idx" &
	running=$((running + 1))
	idx=$((idx + 1))
	if [ "$running" -ge "$LANES" ]; then
		wait -n || true
		running=$((running - 1))
	fi
done
wait || true

fail_seen=0 env_seen=0 budget_seen=0
for ((i = 0; i < idx; i++)); do
	[ ! -f "$tmp/rows/$i.tsv" ] || cat "$tmp/rows/$i.tsv"
	r=$(cat "$tmp/rc/$i" 2>/dev/null || echo 1)
	case $r in
	0) ;;
	3) env_seen=1 ;;
	5) budget_seen=1 ;;
	*) fail_seen=1 ;;
	esac
done
if [ "$budget_seen" -eq 1 ]; then exit 5; fi
if [ "$fail_seen" -eq 1 ]; then exit 1; fi
if [ "$env_seen" -eq 1 ]; then exit 3; fi
exit 0
