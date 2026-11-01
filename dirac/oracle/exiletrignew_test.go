package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The trigger-own ExileAnyGrave cost lockstep (CR
// 118.1, 603.4): Sandbender Scavengers and Paramecia Coloniex's "when this
// dies, you may exile it. When you do, ..." — gorge's triggered-cost window
// asks pay/decline (payable only while the triggered card is still in a
// graveyard), paying moves that card to exile unasked (one candidate, no
// pick), and declining skips the reflexive body. Deck B's removal fills
// both graveyards and keeps the carriers dying. The expected answer at
// every decision is gorge's.
var kexiletrignewDecks = [2][]cardset.Entry{
	{ // A: B/G
		{Name: "Sandbender Scavengers", Count: 4}, {Name: "Paramecia Coloniex", Count: 4},
		{Name: "Hungry Ghoul", Count: 4}, {Name: "Bake into a Pie", Count: 2}, {Name: "Murder", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{ // B: B/R removal and creatures
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

// kexiletrignewRun plays seeds 1..seeds with the random policy, reporting
// divergences as test errors when report is set (the mutation run expects
// one and stays quiet).
func kexiletrignewRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *dirac.ExileTrigNewStats) {
	t.Helper()
	reg, set, decks := kexiletrignewSetup(t)
	tbl, _, _, err := set.Matchup(kexiletrignewDecks[0], kexiletrignewDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &dirac.ExileTrigNewStats{}
	dirac.DebugExileTrigNewStats = st
	defer func() { dirac.DebugExileTrigNewStats = nil }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kexiletrignew-a", "kexiletrignew-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

// kexiletrignewSetup loads the gorge corpus, the generated set and the decks.
func kexiletrignewSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exiletrignew")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kexiletrignewDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	return reg, set, decks
}

func TestExiletrignewLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kexiletrignewRun(t, 256, true)
	paid, declined := st.Paid.Load(), st.Decline.Load()
	t.Logf("exiletrignew: 256 games, %d divergences, %d stalls; %d paid exiles, %d declines", divs, stalls, paid, declined)
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if testing.Short() {
		return
	}
	if paid < 30 {
		t.Errorf("only %d paid exiles (want >= 30): the pool barely pays the ExileAnyGrave cost", paid)
	}
	if declined < 30 {
		t.Errorf("only %d declines (want >= 30): the pool barely poses the pay/decline election", declined)
	}
}

// TestExiletrignewMutation: exiling the payer's top graveyard card instead
// of the triggered card must make the lockstep diverge.
func TestExiletrignewMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugExileTrigNewAnyCard = true
	defer func() { dirac.DebugExileTrigNewAnyCard = false }()
	if divs, _, _ := kexiletrignewRun(t, 64, false); divs == 0 {
		t.Fatal("DebugExileTrigNewAnyCard: no divergence in 64 seeds")
	}
}
