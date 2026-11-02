package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// sonRun plays seeds 1..n of every matchup of the registered set setName in
// lockstep (card tests share it). check is called at every decision
// (may be nil); mutate, if set, edits the generated table first. It returns
// the number of divergent games; with report, each divergence fails the test,
// else the first stops the run.
func sonRun(t *testing.T, setName string, matchups [][2][]cardset.Entry, n uint64, report bool,
	mutate func(*kop.Table), check func(*GorgeSide, Side) *Divergence) int {
	n2, stalled := sonRunStalls(t, setName, matchups, n, report, mutate, check)
	if stalled != 0 {
		t.Errorf("%d games stalled", stalled)
	}
	return n2
}

// sonRunStalls is sonRun counting the games that stall on a capacity the pool
// may reach, which the test caps by count. An unmodelled stall fails unless
// unmodelledKnown names it, and is then counted the same way.
func sonRunStalls(t *testing.T, setName string, matchups [][2][]cardset.Entry, n uint64, report bool,
	mutate func(*kop.Table), check func(*GorgeSide, Side) *Divergence) (int, int) {
	t.Helper()
	stalled := 0
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(setName)
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range matchups {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{setName + "-a", setName + "-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("matchup %d seed %d: %v", mi, seed, err)
			} else if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalled++
				t.Logf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			} else if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div, stalled
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div, stalled
}
