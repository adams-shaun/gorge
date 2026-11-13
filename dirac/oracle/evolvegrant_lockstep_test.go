package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Tyranid Prime gives its other creatures evolve and Propagator Drone gives
// its creature tokens evolve (CR 702.100a, 613.1f). Experiment One prints
// evolve beside the Prime's grant, so one entering creature evolves it twice
// (CR 113.2c); Raise the Alarm makes the soldier tokens the Drone's grant
// covers.

var kevolveGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Tyranid Prime", Count: 4}, {Name: "Propagator Drone", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Experiment One", Count: 4}, {Name: "Craw Wurm", Count: 2},
		{Name: "Raise the Alarm", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 4}, {Name: "Pacifism", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 16}, {Name: "Island", Count: 6},
	},
}

func TestEvolveGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("evolvegrant")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kevolveGrantDecks {
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
	tbl, _, _, err := set.Matchup(kevolveGrantDecks[0], kevolveGrantDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	games, stalls, granted := 0, 0, 0
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"evolvegrant-a", "evolvegrant-b"}, decks, 1)
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
			if e.Kind == events.GrantTriggerPush && e.Counter == "__kwEvolveGranted" {
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
	t.Logf("%d games, %d stalls; granted evolve triggers %d", games, stalls, granted)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	// the precondition: the grant was reached
	if granted < 10 {
		t.Errorf("granted evolve triggers under-reached: %d (want 10)", granted)
	}
}
