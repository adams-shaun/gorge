package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The as-enters choice of any card name (ETB NameCard with no ValidCards$,
// kgen lower_choosename.go etbChooseName, kernel ExtChooseName B = 1) in
// lockstep over the setsobjects3 fixture pool: Disruptor Flute and Pithing
// Needle name Mind Stone (its draw ability is then forbidden, its mana
// ability kept; the Flute also taxes casting it), Declaration of Naught names
// Grizzly Bears (its {U} counters a Bears spell) and Gideon's Intervention
// names Shock (opponents cannot cast it; its damage to the Intervention's
// controller is prevented). gorge asks the name over every card name of its
// universe at the entry boundary (effects NameChoicesFromListCtx); the
// kernel over the matchup's NameCardsAny.
//
// The expected answer at every decision is gorge's.

type so3Matchup struct {
	card, named string
	lands       []cardset.Entry
}

var so3Matchups = []so3Matchup{
	{card: "Disruptor Flute", named: "Mind Stone", lands: []cardset.Entry{{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}}},
	{card: "Pithing Needle", named: "Mind Stone", lands: []cardset.Entry{{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}}},
	{card: "Declaration of Naught", named: "Grizzly Bears", lands: []cardset.Entry{{Name: "Island", Count: 10}, {Name: "Mountain", Count: 6}}},
	{card: "Gideon's Intervention", named: "Shock", lands: []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}}},
}

func TestSetsObjects3Lockstep(t *testing.T) {
	for _, mu := range so3Matchups {
		t.Run(strings.NewReplacer(" ", "", "'", "").Replace(mu.card), func(t *testing.T) {
			deck := append([]cardset.Entry{
				{Name: mu.card, Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4},
				{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4},
			}, mu.lands...)
			reg, set, decks, tbl := p3Setup(t, "setsobjects3", [2][]cardset.Entry{deck, deck})
			var names [2]map[uint16]bool
			for s := range names {
				names[s] = map[uint16]bool{canon.NameValue(mu.named): true}
			}
			oldC, oldH, oldM := dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld
			dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld = true, 0, 0
			defer func() { dirac.DebugCantActCount, dirac.CantActHeld, dirac.CantActManaHeld = oldC, oldH, oldM }()
			named := 0
			for seed := uint64(1); seed <= 30; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"so3-a", "so3-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kkaiwinnPolicy(seed, names), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
				CloseSide(k)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("%s seed %d: %s", mu.card, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					t.Fatalf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.card, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				for _, o := range g.E.G.Objs {
					if o.ChosenName == mu.named && o.Face() != nil && o.Face().Name == mu.card {
						named++
					}
				}
			}
			t.Logf("%s: %d permanents named %s; activations forbidden %d", mu.card, named, mu.named, dirac.CantActHeld)
			if named < 3 {
				t.Errorf("%s: %d permanents named %s, want 3+: the scenario the lowering must hold was not reached", mu.card, named, mu.named)
			}
			if mu.named == "Mind Stone" && dirac.CantActHeld == 0 {
				t.Errorf("%s: no activation was forbidden", mu.card)
			}
			if dirac.CantActManaHeld != 0 {
				t.Errorf("%s: a mana ability was forbidden %d times", mu.card, dirac.CantActManaHeld)
			}
		})
	}
}
