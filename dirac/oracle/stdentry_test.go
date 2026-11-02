package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryCase is a deck that reaches one entering-object shape, and how to see it
// was reached in gorge's log.
type entryCase struct {
	name    string
	deck    []cardset.Entry
	reached func(g *state.Game, e *events.Event) bool
}

func cardNamed(g *state.Game, id state.ObjID, name string) bool {
	o := g.Obj(id)
	return o != nil && o.Card != nil && o.Card.Faces[0].Name == name
}

var entryCases = []entryCase{
	{
		// CR 303.4f: an Aura a search puts onto the battlefield enchants what its
		// controller chooses, before the library is shuffled.
		name: "search finds an Aura",
		deck: []cardset.Entry{{Name: "Guardian Sunmare", Count: 14}, {Name: "Roadside Assistance", Count: 8},
			{Name: "Plains", Count: 18}},
		reached: func(g *state.Game, e *events.Event) bool {
			return e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZBattlefield && cardNamed(g, e.Obj, "Roadside Assistance")
		},
	},
	{
		// CR 110.2a: a card returned under another player's control is asked about
		// as it enters with that control already folded onto it.
		name: "return under your control asks as it enters",
		deck: []cardset.Entry{{Name: "Coalstoke Gearhulk", Count: 10}, {Name: "Waxen Shapethief", Count: 10},
			{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 4}},
		reached: func(g *state.Game, e *events.Event) bool {
			return e.Kind == events.ControlChange && e.Text == "GainControl" && cardNamed(g, e.Obj, "Waxen Shapethief")
		},
	},
	{
		// CR 707.2: a sacrificed copy's mana value is the copied card's.
		name: "a sacrificed copy keeps the copied mana value",
		deck: []cardset.Entry{{Name: "Repurposing Bay", Count: 8}, {Name: "Waxen Shapethief", Count: 8},
			{Name: "Aether Syphon", Count: 4}, {Name: "Scrap Compactor", Count: 4}, {Name: "Island", Count: 16}},
		reached: func(g *state.Game, e *events.Event) bool {
			return e.Kind == events.MoveZone && e.Text == "sacrificed" && cardNamed(g, e.Obj, "Waxen Shapethief")
		},
	},
}

func TestEnteringObjectsLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.SynthByName("std-dft")
	if err != nil {
		t.Skip(err)
	}
	set = cardset.AllowUnverified(set)
	for _, tc := range entryCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var deck []*cards.Card
			for _, e := range tc.deck {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					deck = append(deck, c)
				}
			}
			tbl, _, _, err := set.Matchup(tc.deck, tc.deck)
			if err != nil {
				t.Fatal(err)
			}
			reached := 0
			for seed := uint64(1); seed <= 48; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"entry-a", "entry-b"}, [2][]*cards.Card{deck, deck}, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				var last *GorgeSide
				pol := RandomPolicy(seed)
				res := Lockstep(m, seed, k, PolicyFunc(func(c *PickCtx) Canon {
					last = c.G
					return pol.Pick(c)
				}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				}
				if last != nil {
					for i := range last.E.L.Events {
						if tc.reached(last.E.G, &last.E.L.Events[i]) {
							reached++
						}
					}
				}
			}
			t.Logf("%d games reached it %d times", 48, reached)
			if reached == 0 {
				t.Fatal("no game reached the shape")
			}
		})
	}
}
