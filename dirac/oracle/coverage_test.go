package oracle

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var updateCoverageGolden = flag.Bool("update-coverage-golden", false, "rewrite testdata/coverage_keys.golden")

// coverageGoldenCards are one card of each inventory shape: a creature with
// an activated ability, an ETB trigger, a planeswalker, a land with two or
// more mana abilities, a modal DFC, and a card with a bookkeeping trigger
// (no key) beside real ones.
var coverageGoldenCards = []string{
	"Prodigal Sorcerer",
	"Elvish Visionary",
	"Ajani, Caller of the Pride",
	"Llanowar Wastes",
	"Valakut Awakening // Valakut Stoneforge",
	"Chrome Mox",
}

// TestCoverageKeysMatchCardfuzz pins the ported inventory (cardfuzz's
// abilityInventory) on a fixed card list against a committed golden.
func TestCoverageKeysMatchCardfuzz(t *testing.T) {
	c, _ := testCorpus(t)
	var b strings.Builder
	for _, n := range coverageGoldenCards {
		cd, ok := c.Reg.Lookup(n)
		if !ok {
			t.Fatalf("%q not in the corpus", n)
		}
		fmt.Fprintf(&b, "%s\t%s\n", n, strings.Join(CoverageKeys(cd), ","))
	}
	path := filepath.Join("testdata", "coverage_keys.golden")
	if *updateCoverageGolden {
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != string(want) {
		t.Fatalf("coverage keys differ from %s:\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// coverageFuzz plays the first 4 FDN pairings gorge-vs-gorge under pol
// with coverage on.
func coverageFuzz(t *testing.T, pol func(uint64) Policy, workers int, finish bool) FuzzSummary {
	t.Helper()
	c, pool := testCorpus(t)
	return Fuzz(c, Pairings(pool, 4, 20261026), GorgeKernel(Config{}), FuzzConfig{Policy: pol, Workers: workers, Coverage: finish}, nil)
}

func abilityKeysUsed(cov *Coverage) (n int) {
	for _, cc := range cov.Cards {
		for k, v := range cc.Used {
			if v > 0 && strings.Contains(k, "/a") {
				n++
			}
		}
	}
	return n
}

// TestExplorePolicyGorgeSelfCheck: the explore bot drives gorge against
// itself without a divergence, and exercises strictly more activated
// ability keys than the heuristic bot on the same pairs.
func TestExplorePolicyGorgeSelfCheck(t *testing.T) {
	ex := coverageFuzz(t, ExplorePolicy, 2, true)
	if ex.Diverged != 0 || ex.Errors != 0 {
		t.Fatalf("explore self-check: %d diverged, %d errors (%v)", ex.Diverged, ex.Errors, ex.ByClass)
	}
	he := coverageFuzz(t, HeuristicPolicy, 2, true)
	ne, nh := abilityKeysUsed(ex.Coverage), abilityKeysUsed(he.Coverage)
	te := ex.Coverage.Totals()
	t.Logf("explore: %d f*/a* keys used, totals %+v, fallbacks %d/%d; heuristic: %d f*/a* keys", ne, te, ex.Coverage.Fallbacks, ex.Coverage.Decisions, nh)
	if ne == 0 {
		t.Fatal("explore credited no activated ability key")
	}
	if ne <= nh {
		t.Fatalf("explore used %d activated ability keys, heuristic %d: want strictly more", ne, nh)
	}
}

// TestCoverageDeterministicAcrossWorkers: the merged TSV is byte-identical
// with 1 and 2 workers.
func TestCoverageDeterministicAcrossWorkers(t *testing.T) {
	var tsv [2]bytes.Buffer
	for i, w := range []int{1, 2} {
		s := coverageFuzz(t, ExplorePolicy, w, true)
		if err := s.Coverage.WriteTSV(&tsv[i]); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(tsv[0].Bytes(), tsv[1].Bytes()) {
		t.Fatalf("coverage TSV differs across workers:\n%s\n---\n%s", tsv[0].String(), tsv[1].String())
	}
	if strings.Count(tsv[0].String(), "\n") < 2 {
		t.Fatalf("coverage TSV has no rows:\n%s", tsv[0].String())
	}
}

// TestCoverageOffIsFree: Fuzz with coverage off (RunConfig.Finish nil)
// plays exactly the games it plays with the hooks set: same actions and
// divergences on 2 pairs.
func TestCoverageOffIsFree(t *testing.T) {
	c, pool := testCorpus(t)
	pairs := Pairings(pool, 2, 20261026)
	var res [2][]Result
	for i, on := range []bool{false, true} {
		res[i] = make([]Result, len(pairs))
		s := Fuzz(c, pairs, GorgeKernel(Config{}), FuzzConfig{Policy: ExplorePolicy, Coverage: on}, func(j int, _ Pairing, r *Result) { res[i][j] = *r })
		if on != (s.Coverage != nil) {
			t.Fatalf("coverage %v: summary coverage %v", on, s.Coverage)
		}
	}
	for j := range pairs {
		a, b := res[0][j], res[1][j]
		if !reflect.DeepEqual(a.Actions, b.Actions) || !reflect.DeepEqual(a.Div, b.Div) || a.Decisions != b.Decisions || a.Winner != b.Winner {
			t.Fatalf("pair %d: coverage hooks changed the game (%d vs %d actions, div %v vs %v)", j, len(a.Actions), len(b.Actions), a.Div, b.Div)
		}
	}
}
