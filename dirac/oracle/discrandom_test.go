package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The random-discard lockstep: Curse-Marred Demon's and
// Cool but Rude's resolution `Discard | Mode$ Random`, plus Mind Knives,
// Desperate Ravings and Hymn to Tourach (the non-Std carriers), beside Fiery
// Temper so the discarded card's identity matters (madness). Gorge's
// effDiscardRandom draws the engine RNG once per pick from the ordered
// eligible hand, including a one-card hand, and the kernel must match.
var kdiscrandomDecks = [2][]cardset.Entry{
	{ // A: B/R, the Standard carriers, with Fiery Temper beside Cool but
		// Rude so a madness discard reaches its Mode$ Discarded trigger: the
		// kernel gates that trigger on the madness branch because gorge's
		// replacement rewrites the move Text to "discarded (madness exile)",
		// which events.IsDiscard rejects (GORGE-FINDINGS.md).
		{Name: "Curse-Marred Demon", Count: 4}, {Name: "Cool but Rude", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Fiery Temper", Count: 4},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 12},
	},
	{ // B: a generic B/R deck with the non-Std random-discard spells
		{Name: "Mind Knives", Count: 4}, {Name: "Desperate Ravings", Count: 4},
		{Name: "Hymn to Tourach", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
}

// kdiscrandomRun plays seeds 1..seeds with the reach counters, reporting
// divergences as test errors when report is set (the mutation run expects
// one and stays quiet). It returns the multi-card and one-card picks.
func kdiscrandomRun(t *testing.T, seeds uint64, report bool) (divs, stalls, one, multi int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("discrandom")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdiscrandomDecks {
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
	tbl, _, _, err := set.Matchup(kdiscrandomDecks[0], kdiscrandomDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugDiscRandomHand1, dirac.DebugDiscRandomHand2 = 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdiscrandom-a", "kdiscrandom-b"}, decks, 1)
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
	return divs, stalls, dirac.DebugDiscRandomHand1, dirac.DebugDiscRandomHand2
}

func TestDiscrandomLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, one, multi := kdiscrandomRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; random discards: %d from a hand of >=2, %d from a hand of exactly 1",
		256, divs, stalls, multi, one)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if multi < 40 {
		t.Errorf("only %d random discards from a hand of >=2: the pool does not reach the mechanism", multi)
	}
	if one < 5 {
		t.Errorf("only %d random discards from a hand of exactly 1: the one-card draw is unexercised", one)
	}
}

// TestDiscrandomMutation: picking hand[0] instead of the RNG's card must
// make the lockstep diverge.
func TestDiscrandomMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDiscRandomFirst = true
	defer func() { dirac.DebugDiscRandomFirst = false }()
	if divs, _, _, _ := kdiscrandomRun(t, 64, false); divs == 0 {
		t.Fatal("DebugDiscRandomFirst: no divergence in 64 seeds")
	}
}
