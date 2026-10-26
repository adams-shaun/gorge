package rules

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The set-scoped parameter census: the same scan and labels as
// TestEveryRepoDeckParamsAreRead, walked over every card of each Standard
// program set listed in testdata/sets/<SET>.txt. Baselines live in
// testdata/paramcensus-sets/<SET>.tsv ("<card>\t<labels, comma-joined>",
// sorted by card); PARAMCENSUS_SETS_REGEN=1 rewrites them from the build.

func readSetNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if s := strings.TrimSpace(sc.Text()); s != "" {
			out = append(out, s)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func readSetBaseline(t *testing.T, path string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		name, labels, ok := strings.Cut(line, "\t")
		if !ok || labels == "" {
			t.Errorf("%s: line %q is not <card>\\t<labels>", path, line)
			continue
		}
		out[name] = strings.Split(labels, ",")
	}
	return out
}

func writeSetBaseline(t *testing.T, path string, got map[string][]string) {
	t.Helper()
	names := make([]string, 0, len(got))
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s\t%s\n", n, strings.Join(got[n], ","))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEveryStandardSetParamsAreRead(t *testing.T) {
	_, d := measureParamCensus(t, nil)
	reg := testutil.CorpusRegistry(t)
	lists, err := filepath.Glob(filepath.Join("testdata", "sets", "*.txt"))
	if err != nil || len(lists) == 0 {
		t.Fatalf("no set lists: %v", err)
	}
	sort.Strings(lists)
	regen := os.Getenv("PARAMCENSUS_SETS_REGEN") != ""
	totCards, totBad := 0, 0
	totLabels := map[string]bool{}
	cardsPerLabel := map[string]int{}
	setsPerLabel := map[string]map[string]bool{}
	slowest, slowestSet := time.Duration(0), ""
	for _, list := range lists {
		set := strings.TrimSuffix(filepath.Base(list), ".txt")
		names := readSetNames(t, list)
		start := time.Now()
		got := map[string][]string{}
		var missing []string
		for _, name := range names {
			c, ok := reg.Lookup(name)
			if !ok {
				missing = append(missing, name)
				continue
			}
			if labels := cardCensusLabels(c, d, nil); len(labels) > 0 {
				got[name] = labels
			}
		}
		if el := time.Since(start); el > slowest {
			slowest, slowestSet = el, set
		}
		setLabels := map[string]bool{}
		for _, ls := range got {
			for _, l := range ls {
				setLabels[l] = true
				totLabels[l] = true
				cardsPerLabel[l]++
				if setsPerLabel[l] == nil {
					setsPerLabel[l] = map[string]bool{}
				}
				setsPerLabel[l][set] = true
			}
		}
		totCards += len(names)
		totBad += len(got)
		t.Logf("%s: cards=%d cards with unread params=%d distinct labels=%d", set, len(names), len(got), len(setLabels))
		if len(missing) > 0 {
			t.Errorf("%s: %d card names the corpus lacks: %v", set, len(missing), missing)
		}
		tsv := filepath.Join("testdata", "paramcensus-sets", set+".tsv")
		if regen {
			writeSetBaseline(t, tsv, got)
			continue
		}
		want := readSetBaseline(t, tsv)
		for card, g := range got {
			w, ok := want[card]
			switch {
			case !ok:
				t.Errorf("%s: %s carries unread params %v, not in %s -- new gap", set, card, g, tsv)
			case !sameSet(w, g):
				t.Errorf("%s: %s: baseline says %v, measured %v -- stale baseline, update it", set, card, w, g)
			}
		}
		for card, w := range want {
			if _, still := got[card]; !still {
				t.Errorf("%s: %s now reads everything the baseline listed (%v) -- stale, delete the line", set, card, w)
			}
		}
	}
	t.Logf("TOTAL: sets=%d cards=%d cards with unread params=%d distinct labels=%d (slowest set %s %v)",
		len(lists), totCards, totBad, len(totLabels), slowestSet, slowest)
	type lc struct {
		l    string
		n, s int
	}
	var top []lc
	for l, n := range cardsPerLabel {
		top = append(top, lc{l, n, len(setsPerLabel[l])})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].n != top[j].n {
			return top[i].n > top[j].n
		}
		return top[i].l < top[j].l
	})
	for i, e := range top {
		if i == 20 {
			break
		}
		t.Logf("top label %2d: %-60s cards=%d sets=%d", i+1, e.l, e.n, e.s)
	}
}
