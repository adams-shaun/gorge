package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The blue-symbol lockstep: Namor the Sub-Mariner makes a Merfolk token for each
// blue mana symbol in the printed cost of a noncreature spell its controller
// casts (ManaCostPartialBlue, TriggeredCard$ChromaSource.Blue), a Phyrexian
// {U/P} symbol and creature spells included in the sweep of the decks below.
var kchromaDecks = [2][]cardset.Entry{
	{
		{Name: "Namor the Sub-Mariner", Count: 4}, {Name: "Island", Count: 14},
		{Name: "Mountain", Count: 2}, {Name: "Opt", Count: 3}, {Name: "Counterspell", Count: 3},
		{Name: "Unsummon", Count: 2}, {Name: "Gitaxian Probe", Count: 3}, {Name: "Brainstorm", Count: 2},
		{Name: "Divination", Count: 3}, {Name: "Suntail Hawk", Count: 2}, {Name: "Lightning Bolt", Count: 2},
	},
	{
		{Name: "Namor the Sub-Mariner", Count: 2}, {Name: "Island", Count: 12},
		{Name: "Mountain", Count: 4}, {Name: "Mana Leak", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Giant Growth", Count: 3},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Unsummon", Count: 2},
	},
}

func TestChromaLockstep(t *testing.T) { kchromaRun(t, false) }

// TestChromaMutation: with Namor counting white symbols instead of blue the
// lockstep must diverge (gorge makes a token for each blue symbol).
func TestChromaMutation(t *testing.T) { kchromaRun(t, true) }

func kchromaRun(t *testing.T, mutant bool) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("chroma")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kchromaDecks {
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
	tbl, _, _, err := set.Matchup(kchromaDecks[0], kchromaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutant {
		for i := range tbl.Exprs {
			if tbl.Exprs[i].Kind == kop.ExChromaOf {
				tbl.Exprs[i].N = 0
			}
		}
	}
	games, divs, stalls, tokens := 0, 0, 0, 0
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kchroma-a", "kchroma-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if d := res.Div; d != nil {
			divs++
			if mutant {
				return
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		for _, e := range g.E.L.Events {
			if e.Kind == events.TokenCreate {
				tokens++
			}
		}
	}
	if mutant {
		t.Fatal("no divergence in 300 seeds with Namor counting the wrong colour")
	}
	t.Logf("%d games, %d divergences, %d stalls; %d tokens", games, divs, stalls, tokens)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tokens < 30 {
		t.Errorf("only %d tokens, want >= 30", tokens)
	}
}
