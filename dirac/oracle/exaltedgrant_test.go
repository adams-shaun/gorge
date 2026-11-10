package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Sublime Archangel gives the other creatures its controller has exalted
// (CR 702.83a-b, 113.2c): a lone attacker triggers once per exalted
// permanent and per grant, +1/+1 each. Akrasan Squire prints exalted beside
// the Archangel's printed and granted instances, so the lone attacker's
// triggers come from printed and granted holders at once.

var kexaltedGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Sublime Archangel", Count: 4}, {Name: "Akrasan Squire", Count: 5}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Gray Ogre", Count: 5}, {Name: "Plains", Count: 20},
	},
	{
		{Name: "Sublime Archangel", Count: 3}, {Name: "Akrasan Squire", Count: 3}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Gray Ogre", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 14}, {Name: "Mountain", Count: 6},
	},
}

func TestExaltedGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exaltedgrant")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kexaltedGrantDecks {
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
	tbl, _, _, err := set.Matchup(kexaltedGrantDecks[0], kexaltedGrantDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	games, stalls, granted := 0, 0, 0
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"exaltedgrant-a", "exaltedgrant-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kinnerlightPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		for _, e := range g.E.L.Events {
			if e.Kind == events.KeywordTriggerPush && e.Counter == "__kwExaltedGranted" {
				granted++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d stalls; granted exalted triggers %d", games, stalls, granted)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	// the precondition: the grant was reached
	if granted < 10 {
		t.Errorf("granted exalted triggers under-reached: %d (want 10)", granted)
	}
}
