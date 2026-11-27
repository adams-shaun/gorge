#!/usr/bin/env bash
# ci-pkg-times.sh <run-id> [<run-id>...]: print "<package>\t<seconds>" for every package the given CI runs' test
# shards passed, the input ci_shard.sh balances on (scripts/ci-pkg-times.tsv).
#
# Per package the value is the median over the runs that printed it; packages none of them printed keep their
# earlier value from scripts/ci-pkg-times.tsv, so the output can overwrite that file directly. The "Golden chain
# heads" step is excluded: the pinned TestHeads call measures one test of engine/rules, not the package.
# A split package (ci-splits.tsv) prints two ok lines per run, one per half; the bigger is the package's whole
# entry, the smaller its #A entry (the ratio the tsv header records).
set -euo pipefail
echo "# ci-pkg-times.tsv: measured seconds per package (go test's \"ok\" line; median over the runs given, a package none of them ran keeps its earlier value). ci_shard.sh balances shards on it."
echo "# Refresh: scripts/ci-pkg-times.sh <run-id> [<run-id>...] > scripts/ci-pkg-times.tsv"
here=$(dirname "$0")
raw=$(mktemp)
trap 'rm -f "$raw"' EXIT
run=0
for r in "$@"; do
	run=$((run + 1))
	echo "!$run" >>"$raw"
	gh run view "$r" --log \
		| awk -F'\t' '$1 ~ /^test \([0-9]+\)$/ && $2 != "Golden chain heads"' \
		| grep -E $'ok  \tgithub\\.com' \
		| sed -E 's/.*ok +\t?(github.com[^ \t]*)[ \t]+([0-9.]+)s.*/\1\t\2/' \
		| awk -F'\t' '{printf "%s\t%.0f\n", $1, $2}' >>"$raw" || {
		echo "ci-pkg-times: run $r printed no test-shard ok lines (GitHub retained no logs for it); drop it from the refresh" >&2
		exit 1
	}
done
awk -F'\t' -v raw="$raw" -v splits="$here/ci-splits.tsv" '
BEGIN {
	while ((getline line < splits) > 0) {
		split(line, f, "\t")
		if (f[1] != "" && f[1] !~ /^#/) splitpkg[f[1]] = 1
	}
}
FNR == NR { if ($0 !~ /^#/ && NF >= 2) old[$1] = $2; next }
/^!/ { run++; next }
NF >= 2 {
	c[pkg = $1, run]++
	v = vals[pkg, run]
	vals[pkg, run] = (v == "" ? $2 : v " " $2 + 0)
}
function median(str,    n, a, i, j, t) {
	n = split(str, a, " ")
	for (i = 2; i <= n; i++) { t = a[i]; for (j = i - 1; j >= 1 && a[j] > t; j--) a[j + 1] = a[j]; a[j + 1] = t }
	if (n % 2) return a[(n + 1) / 2]
	return (a[n / 2] + a[n / 2 + 1]) / 2
}
END {
	for (k in c) {
		split(k, kk, SUBSEP); pkg = kk[1]; r = kk[2]
		n = split(vals[k], a, " ")
		if (pkg in splitpkg) {
			if (n != 2) { print "ci-pkg-times: split package " pkg " printed " n " of its two halves in run " r > "/dev/stderr"; bad = 1; continue }
			hi = a[1] > a[2] ? a[1] : a[2]; lo = a[1] > a[2] ? a[2] : a[1]
			one[pkg] = one[pkg] == "" ? hi : one[pkg] " " hi
			akey = pkg "#A"
			one[akey] = one[akey] == "" ? lo : one[akey] " " lo
		} else {
			for (i = 1; i <= n; i++) one[pkg] = one[pkg] == "" ? a[i] : one[pkg] " " a[i]
		}
	}
	if (bad) exit 2
	for (pkg in old) key[++nk] = pkg
	for (pkg in one) if (!(pkg in old)) key[++nk] = pkg
	for (i = 2; i <= nk; i++) { t = key[i]; for (j = i - 1; j >= 1 && key[j] > t; j--) key[j + 1] = key[j]; key[j + 1] = t }
	for (i = 1; i <= nk; i++) {
		pkg = key[i]
		printf "%s\t%d\n", pkg, int(one[pkg] != "" ? median(one[pkg]) + 0.5 : old[pkg] + 0.5)
	}
}
' "$here/ci-pkg-times.tsv" "$raw"
