package kgen

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/forge"
)

// typedShareFile commits each pool's typed share, one "pool<TAB>share" row a
// pool, sorted by pool: a share may only rise.
const typedShareFile = "testdata/typed-filter-share.tsv"

// typedShareUpdateEnv=1 rewrites a measured pool's row instead of failing on
// a rise or a missing row. A fall fails in both modes. The package has no
// golden flag; the rewrite lands only in a local run (GORGE_REMOTE_OFF=1).
const typedShareUpdateEnv = "TYPED_SHARE_UPDATE"

// typedShareSlack is how far, in thousandths, a measured share may stand
// above its committed value before the rise must be committed.
const typedShareSlack = 5

// typedSharePools are the pools typedShareFile holds, sorted.
var typedSharePools = []string{"blb", "edh", "fdn", "hosted-cmdr"}

var typedShareRow = regexp.MustCompile(`^([a-z0-9-]+)\t([01])\.([0-9]{3})$`)

// typedShareText is a share of milli thousandths with three decimals.
func typedShareText(milli int) string { return fmt.Sprintf("%d.%03d", milli/1000, milli%1000) }

// typedShareLine is the file row for a share of milli thousandths.
func typedShareLine(pool string, milli int) string { return pool + "\t" + typedShareText(milli) }

// parseTypedShares reads typedShareFile's rows as pool to thousandths. A row
// of another shape, or one out of pool order, is an error.
func parseTypedShares(b []byte) (map[string]int, error) {
	out := map[string]int{}
	prev := ""
	for i, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		m := typedShareRow.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("line %d %q: want pool<TAB>share with three decimals", i+1, line)
		}
		if m[1] <= prev {
			return nil, fmt.Errorf("line %d: pool %s after %s: rows are sorted by pool, one a pool", i+1, m[1], prev)
		}
		prev = m[1]
		milli, err := strconv.Atoi(m[2] + m[3])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		out[m[1]] = milli
	}
	return out, nil
}

// checkTypedShare holds a pool that took the typed path on typed of its total
// filter reads to its committed share. The row it asks for is rounded down,
// so a committed share is never above the one it was measured from.
func checkTypedShare(committed map[string]int, pool string, typed, total int) error {
	line := typedShareLine(pool, typed*1000/total)
	c, ok := committed[pool]
	switch {
	case !ok:
		return fmt.Errorf("%s: no committed typed share: commit %q to %s", pool, line, typedShareFile)
	case typed*1000 < c*total:
		return fmt.Errorf("%s: typed share %.4f (%d of %d) fell below the committed %q: the share may only rise",
			pool, float64(typed)/float64(total), typed, total, typedShareLine(pool, c))
	case typed*1000 > (c+typedShareSlack)*total:
		return fmt.Errorf("%s: typed share %.4f is more than 0.%03d above the committed %q: commit %q to %s (or run with %s=1)",
			pool, float64(typed)/float64(total), typedShareSlack, typedShareLine(pool, c), line, typedShareFile, typedShareUpdateEnv)
	}
	return nil
}

// ratchetTypedShare checks a measured pool against the share file at path.
// With update set it writes the pool's row, keeping the others, unless the
// share fell.
func ratchetTypedShare(path string, update bool, pool string, typed, total int) error {
	b, err := os.ReadFile(path)
	if err != nil && !(update && errors.Is(err, fs.ErrNotExist)) {
		return err
	}
	committed := map[string]int{}
	if len(b) > 0 {
		if committed, err = parseTypedShares(b); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if !update {
		return checkTypedShare(committed, pool, typed, total)
	}
	if c, ok := committed[pool]; ok && typed*1000 < c*total {
		return checkTypedShare(committed, pool, typed, total)
	}
	committed[pool] = typed * 1000 / total
	var out strings.Builder
	for _, p := range slices.Sorted(maps.Keys(committed)) {
		out.WriteString(typedShareLine(p, committed[p]) + "\n")
	}
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

// holdTypedShare is the ratchet every measured pool goes through.
func holdTypedShare(t *testing.T, pool string, typed, total int) {
	t.Helper()
	if err := ratchetTypedShare(typedShareFile, os.Getenv(typedShareUpdateEnv) == "1", pool, typed, total); err != nil {
		t.Error(err)
	}
}

// refusalLines is one line per front-end refusal kind st counted, by count
// descending then kind.
func refusalLines(st filterStats) []string {
	var kinds []forge.RefusalKind
	for k, n := range st.refusedBy {
		if n != 0 {
			kinds = append(kinds, forge.RefusalKind(k))
		}
	}
	slices.SortFunc(kinds, func(a, b forge.RefusalKind) int {
		return cmp.Or(cmp.Compare(st.refusedBy[b], st.refusedBy[a]), cmp.Compare(a, b))
	})
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = fmt.Sprintf("front end refused %d as %s", st.refusedBy[k], k)
	}
	return out
}

// firstNameDiff is the first card on one sorted-alike list and not the other.
func firstNameDiff(a, b []string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	at := func(s []string) string {
		if i < len(s) {
			return s[i]
		}
		return "(end)"
	}
	return fmt.Sprintf("index %d: string %q, typed %q", i, at(a), at(b))
}

// TestTypedFilterCommanderPools builds the Commander pools with the string
// reader and with the typed reader: the tables and the supported lists must be
// the same, and each pool's typed share is held to typedShareFile.
func TestTypedFilterCommanderPools(t *testing.T) {
	cardsDir := tableCorpus(t)
	for _, name := range []string{"hosted-cmdr", "edh"} {
		t.Run(name, func(t *testing.T) {
			if !commanderPools[name] {
				t.Fatalf("%s is not a Commander pool", name)
			}
			strTab, strSup, strSt := typedPoolTable(t, name, cardsDir, filterReadString)
			typTab, typSup, st := typedPoolTable(t, name, cardsDir, filterReadTyped)
			if strSt != (filterStats{}) {
				t.Fatalf("the string reader counted typed reads: %+v", strSt)
			}
			if len(strSup) == 0 {
				t.Fatal("the string reader supports no card: the comparison ran on nothing")
			}
			if !bytes.Equal(strTab, typTab) {
				t.Errorf("table differs: %d bytes string, %d typed, first diff at byte %d", len(strTab), len(typTab), firstDiff(strTab, typTab))
			}
			if !slices.Equal(strSup, typSup) {
				t.Errorf("supported lists differ: %d string, %d typed, first at %s", len(strSup), len(typSup), firstNameDiff(strSup, typSup))
			}
			total := st.typed + st.fallbackFrontEnd + st.fallbackReader
			if total == 0 {
				t.Fatal("no filter read reached parseFilterText")
			}
			t.Logf("%s: %d supported, table %d bytes", name, len(typSup), len(typTab))
			// the share is logged as typedShareFile writes it, rounded down
			t.Logf("%s: typed %d, fallbackFrontEnd %d, fallbackReader %d, typed share %s", name, st.typed, st.fallbackFrontEnd, st.fallbackReader, typedShareText(st.typed*1000/total))
			for _, line := range refusalLines(st) {
				t.Logf("%s: %s", name, line)
			}
			holdTypedShare(t, name, st.typed, total)
		})
	}
}

// TestTypedFilterShareFile holds the committed share file to its shape, and
// drives the ratchet with made-up counts.
func TestTypedFilterShareFile(t *testing.T) {
	b, err := os.ReadFile(typedShareFile)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := parseTypedShares(b) // sorted, three decimals
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(committed)); !slices.Equal(got, typedSharePools) {
		t.Errorf("%s holds pools %q, want %q", typedShareFile, got, typedSharePools)
	}

	for _, bad := range []string{
		"fdn\t0.945\nblb\t0.953\n", // unsorted
		"blb\t0.953\nblb\t0.953\n", // repeated
		"blb\t0.95\n",              // two decimals
		"blb\t0.9531\n",            // four decimals
		"blb 0.953\n",              // no tab
		"",                         // empty
	} {
		if _, err := parseTypedShares([]byte(bad)); err == nil {
			t.Errorf("parseTypedShares(%q): no error", bad)
		}
	}

	fake := map[string]int{"edh": 900}
	for _, tc := range []struct {
		name         string
		pool         string
		typed, total int
		want         []string // nil: passes
	}{
		{"equal", "edh", 900, 1000, nil},
		{"rise within slack", "edh", 905, 1000, nil},
		{"fall", "edh", 899, 1000, []string{"edh", "fell below", "edh\t0.900"}},
		{"fall under a thousandth", "edh", 89999, 100000, []string{"edh", "fell below"}},
		{"rise past slack", "edh", 906, 1000, []string{"edh", `edh\t0.906`}},
		{"rise just past slack, row rounded down", "edh", 90599, 100000, []string{"edh", `edh\t0.905`}},
		{"no row", "blb", 1, 3, []string{"blb", `blb\t0.333`}},
	} {
		err := checkTypedShare(fake, tc.pool, tc.typed, tc.total)
		switch {
		case tc.want == nil && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != nil && err == nil:
			t.Errorf("%s: no error", tc.name)
		case tc.want != nil:
			// the row to commit is printed quoted, so its tab reads \t
			msg := strings.ReplaceAll(err.Error(), `\t`, "\t")
			for _, w := range tc.want {
				if !strings.Contains(msg, strings.ReplaceAll(w, `\t`, "\t")) {
					t.Errorf("%s: %q does not name %q", tc.name, err, w)
				}
			}
		}
	}

	// update mode writes the measured row, keeps the others and never lowers one
	path := filepath.Join(t.TempDir(), "share.tsv")
	read := func() string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if err := ratchetTypedShare(path, false, "fdn", 1, 2); err == nil {
		t.Error("a missing file passes without update")
	}
	for _, step := range []struct {
		pool  string
		typed int
	}{{"fdn", 500}, {"blb", 700}, {"fdn", 600}} {
		if err := ratchetTypedShare(path, true, step.pool, step.typed, 1000); err != nil {
			t.Fatal(err)
		}
	}
	const want = "blb\t0.700\nfdn\t0.600\n"
	if got := read(); got != want {
		t.Fatalf("updated file %q, want %q", got, want)
	}
	if err := ratchetTypedShare(path, true, "fdn", 599, 1000); err == nil {
		t.Error("update accepted a fall")
	}
	if got := read(); got != want {
		t.Errorf("a refused fall rewrote the file to %q", got)
	}

	// the refusal lines: by count descending, then kind
	var st filterStats
	st.refusedBy[forge.RefuseNestedFilter] = 2
	st.refusedBy[forge.RefuseUnknownWord] = 2
	st.refusedBy[forge.RefuseBoundNamesSVar] = 7
	wantLines := []string{
		"front end refused 7 as bound-names-svar",
		"front end refused 2 as unknown-word",
		"front end refused 2 as nested-filter",
	}
	if got := refusalLines(st); !slices.Equal(got, wantLines) {
		t.Errorf("refusal lines %q, want %q", got, wantLines)
	}
}
