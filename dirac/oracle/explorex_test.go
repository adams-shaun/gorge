package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The explores-X-times lockstep: Jadelight Spelunker enters with X counters
// of explore, each a reveal that takes a land or poses the graveyard-or-top ask
// for a nonland card, X read when the trigger resolves (gorge effExplore).
var kexploreDecks = [2][]cardset.Entry{
	{
		{Name: "Jadelight Spelunker", Count: 6}, {Name: "Forest", Count: 12},
		{Name: "Island", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Opt", Count: 3}, {Name: "Lightning Bolt", Count: 3},
	},
	{
		{Name: "Jadelight Spelunker", Count: 4}, {Name: "Forest", Count: 10},
		{Name: "Mountain", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 2},
	},
}

func TestExploreXLockstep(t *testing.T) { kexploreRun(t, false) }

// TestExploreXMutation: with the explore repeated once instead of X times the
// lockstep must diverge (gorge explores X times).
func TestExploreXMutation(t *testing.T) { kexploreRun(t, true) }

func kexploreRun(t *testing.T, mutant bool) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("explorex")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kexploreDecks {
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
	tbl, _, _, err := set.Matchup(kexploreDecks[0], kexploreDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutant {
		for c := range tbl.Cards {
			for a := range tbl.Cards[c].Abilities {
				prog := tbl.Cards[c].Abilities[a].Prog
				for i := range prog {
					if prog[i].Op == kop.OpRepeat {
						prog[i].A = kop.Const(1)
					}
				}
			}
		}
	}
	games, divs, stalls, explores, multi := 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kexplore-a", "kexplore-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 60})
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
		run := 0
		for _, e := range g.E.L.Events {
			switch e.Kind {
			case events.TriggerPush:
				run = 0
			case events.Explore:
				explores++
				if run++; run == 2 {
					multi++ // a second explore with no trigger between: X >= 2
				}
			}
		}
	}
	if mutant {
		t.Fatal("no divergence in 300 seeds with the explore repeated once")
	}
	t.Logf("%d games, %d divergences, %d stalls; %d explores, %d in a run", games, divs, stalls, explores, multi)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if multi < 20 {
		t.Errorf("only %d repeated explores, want >= 20", multi)
	}
}
