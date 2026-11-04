package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Stray from History lockstep: a charm whose first mode returns up to two
// creature cards from the graveyard (target slot 0) and whose second mode
// targets an opponent (slot 1) whose hand then loses a nonland card the caster
// chooses. The hand owner is the second mode's own player slot, not the
// ability-wide target count: the kernel must exile from the same hand, the same
// card, as gorge at every decision.

var kstrayhistoryDecks = [2][]cardset.Entry{
	{
		{Name: "Stray from History", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 9},
	},
}

func runStrayHistory(t *testing.T, seeds int, mutate bool) (games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("strayhistory")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugStrayHandOwnerSlot
	dirac.DebugStrayHandOwnerSlot = mutate
	defer func() { dirac.DebugStrayHandOwnerSlot = old }()
	var decks [2][]*cards.Card
	for s, d := range kstrayhistoryDecks {
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
	tbl, _, _, err := set.Matchup(kstrayhistoryDecks[0], kstrayhistoryDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstray-a", "kstray-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts whenever it can
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestStrayFromHistoryLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls := runStrayHistory(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
}

// TestStrayFromHistoryMutation reads the hand owner from the wrong target
// slot: the games must diverge. A green mutation means the lockstep never
// reaches the second mode's exile.
func TestStrayFromHistoryMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runStrayHistory(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation read the wrong hand owner slot in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
