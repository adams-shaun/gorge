package oracle

// SCRATCH hunt for the Vivid Crag Flags divergence (ticket projection-vivid-crag-flags).
// Not for commit. Plays the vividlands set (the counter-cost lands' own table) with
// the committed lockstep decks, deeper caps and many seeds: the committed test's
// 20-turn cap may simply be too shallow to reach the divergence.

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// deepVividRun is kvividRun with deep caps, returning the first divergence.
func deepVividRun(t *testing.T, pol func(uint64) Policy, n uint64) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vividlands")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvividDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kvividDecks[0], kvividDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"vl-a", "vl-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-20, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Logf("gorge pool %v", g.E.G.Players[0].Pool)
			for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
				if o := g.E.G.Obj(id); o != nil {
					t.Logf("battlefield %d: %q tapped=%v", id, o.Face().Name, o.Tapped)
				}
			}
			return d
		}
	}
	return nil
}

func TestScratchVividDeepHunt(t *testing.T) {
	if isolate(t) {
		return
	}
	if d := deepVividRun(t, pkPolicy, 300); d != nil {
		t.Fatalf("pk diverged at %d: %s %s %v %s", d.At, d.What, d.Field, d.Diff, d.Detail)
	}
	if d := deepVividRun(t, ExplorePolicy, 300); d != nil {
		t.Fatalf("explore diverged at %d: %s %s %v %s", d.At, d.What, d.Field, d.Diff, d.Detail)
	}
	t.Log("vivid deep hunt: clean")
}
