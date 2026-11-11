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

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
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

// TestCoverageCreditsStaticTriggers: a Static$ True trigger with a
// TriggerDescription$ is an ability key, yet it resolves off the stack and
// leaves no ability object. The static-trigger probe credits it: Storied
// Veteran's graveyard-cast rider (f0/t0) is used in a gorge mirror of the
// may-play lockstep's Storied Veteran deck.
func TestCoverageCreditsStaticTriggers(t *testing.T) {
	c, _ := testCorpus(t)
	var es []deck.Entry
	for _, e := range mayPlayDecks[4] {
		es = append(es, deck.Entry{Name: e.Name, Count: e.Count})
	}
	cc := &Corpus{Reg: c.Reg, decks: map[string][]*cards.Card{}}
	cc.AddDecks(map[string]deck.File{"sv-a": {Name: "sv-a", Cards: es}, "sv-b": {Name: "sv-b", Cards: es}})
	var pairs []Pairing
	for i := 0; i < 8; i++ {
		pairs = append(pairs, Pairing{Index: i, A: "sv-a", B: "sv-b", Seed: uint64(i + 1)})
	}
	s := Fuzz(cc, pairs, GorgeKernel(Config{}), FuzzConfig{Policy: RandomPolicy, Coverage: true}, nil)
	if s.Diverged != 0 || s.Errors != 0 || s.Coverage == nil {
		t.Fatalf("gorge mirror: %d diverged, %d errors (%v)", s.Diverged, s.Errors, s.ByClass)
	}
	sv := s.Coverage.Cards["Storied Veteran"]
	if sv == nil {
		t.Fatal("no Storied Veteran row")
	}
	t.Logf("Storied Veteran keys %v used %v", sv.Keys, sv.Used)
	if sv.Used["f0/t0"] == 0 {
		t.Fatalf("Storied Veteran's static rider f0/t0 was never credited: %v", sv.Used)
	}
}

// TestCoverageCreditsDelayedShapeTriggers: a Room's "when you unlock this door"
// reaches the stack in the delayed shape, its body resolved afresh by its Execute$
// name, so no inventory slot's SA is its ability object; the DelayedPush credits it.
func TestCoverageCreditsDelayedShapeTriggers(t *testing.T) {
	c, _ := testCorpus(t)
	const room = "Bottomless Pool // Locker Room"
	cd, ok := c.Reg.Lookup(room)
	if !ok {
		t.Skipf("%q not in the corpus", room)
	}
	es := []deck.Entry{{Name: room, Count: 24}, {Name: "Island", Count: 16}}
	cc := &Corpus{Reg: c.Reg, decks: map[string][]*cards.Card{}}
	cc.AddDecks(map[string]deck.File{"rm-a": {Name: "rm-a", Cards: es}, "rm-b": {Name: "rm-b", Cards: es}})
	var pairs []Pairing
	for i := 0; i < 4; i++ {
		pairs = append(pairs, Pairing{Index: i, A: "rm-a", B: "rm-b", Seed: uint64(i + 1)})
	}
	s := Fuzz(cc, pairs, GorgeKernel(Config{}), FuzzConfig{Policy: RandomPolicy, Coverage: true}, nil)
	if s.Diverged != 0 || s.Errors != 0 || s.Coverage == nil {
		t.Fatalf("gorge mirror: %d diverged, %d errors (%v)", s.Diverged, s.Errors, s.ByClass)
	}
	rc := s.Coverage.Cards[coverageName(cd)]
	if rc == nil {
		t.Fatalf("no %s row", coverageName(cd))
	}
	t.Logf("%s cast %d keys %v used %v", room, rc.Cast, rc.Keys, rc.Used)
	if rc.Cast == 0 {
		t.Fatal("the Room was never cast")
	}
	if rc.Used["f0/t0"] == 0 {
		t.Fatalf("the unlock trigger f0/t0 was never credited: %v", rc.Used)
	}
}
