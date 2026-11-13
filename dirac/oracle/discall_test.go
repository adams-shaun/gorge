package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The DiscardedAll lockstep: "whenever you discard one or
// more cards" triggers once per discard action with the batch's count
// (Mode$ DiscardedAll, gorge's discard batch), against single and
// each-player discards, discard costs and per-card discards.

var kdiscallDecks = [2][]cardset.Entry{
	{
		{Name: "Cryptcaller Chariot", Count: 2}, {Name: "Magmakin Artillerist", Count: 2}, {Name: "Marauding Mako", Count: 2},
		{Name: "Scrounging Skyray", Count: 2}, {Name: "Tinybones, Pocket Nuisance", Count: 2}, {Name: "Inti, Seneschal of the Sun", Count: 2},
		{Name: "Hostile Investigator", Count: 2}, {Name: "Dragon Mage", Count: 1}, {Name: "Chart a Course", Count: 2},
		{Name: "Fearless Swashbuckler", Count: 2}, {Name: "Flamecache Gecko", Count: 1}, {Name: "Wheel of Fortune", Count: 3},
		{Name: "Thirst for Discovery", Count: 3}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Burglar Rat", Count: 4}, {Name: "Duress", Count: 4}, {Name: "Mind Rot", Count: 4}, {Name: "Calamitous Tide", Count: 4},
		{Name: "Tinybones, Pocket Nuisance", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

func runDiscall(t *testing.T, seeds int, mutate bool) (games, divs, stalls, joins, opp, each, unl int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("discall")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugDiscAllPerCard
	dirac.DebugDiscAllPerCard = mutate
	defer func() { dirac.DebugDiscAllPerCard = old }()
	dirac.DebugDiscAllJoins, dirac.DebugDiscAllOpp, dirac.DebugDiscAllEachJoins, dirac.DebugDiscAllUnlessJoins = 0, 0, 0, 0
	var decks [2][]*cards.Card
	for s, d := range kdiscallDecks {
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
	tbl, _, _, err := set.Matchup(kdiscallDecks[0], kdiscallDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdiscall-a", "kdiscall-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, poisonhighPolicy(seed, map[string]int{}, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, dirac.DebugDiscAllJoins, dirac.DebugDiscAllOpp, dirac.DebugDiscAllEachJoins, dirac.DebugDiscAllUnlessJoins
}

func TestDiscallLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, joins, opp, each, unl := runDiscall(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d batched amt>=2 joins, %d opponent-discard matches, %d each-player joins, %d discard-unless joins", games, divs, stalls, joins, opp, each, unl)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if joins < 20 {
		t.Errorf("only %d batched joins (need 20)", joins)
	}
	if opp < 5 {
		t.Errorf("only %d opponent discards seen by a listener (need 5)", opp)
	}
	if each < 5 {
		t.Errorf("only %d each-player batch joins (need 5)", each)
	}
	if unl < 3 {
		t.Errorf("only %d discard-unless batch joins (need 3)", unl)
	}
}

// TestDiscallMutation queues one instance per card: the amounts differ
// from gorge's batches, so the games must diverge.
func TestDiscallMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _, _, _ := runDiscall(t, 64, true)
	if divs == 0 {
		t.Fatalf("per-card mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
