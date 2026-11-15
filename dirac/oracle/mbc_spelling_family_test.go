package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFilters5Lowers plays the Addendum cards the MBC filters-5 family
// lowers, in lockstep against gorge on a table built for exactly their decks
// (kgen refused both before): ConditionPlayerTurn$ True | ConditionPhases$
// Main1,Main2 beside ConditionDefined$ Self | ConditionPresent$ Card.wasCast,
// the "if you cast this spell during your main phase" gate. Random play casts
// the instants in and out of the main phase, so lockstep checks both arms;
// reached counts the Addendum applied:
//
//   - Tail Swipe: a Grizzly Bears at 3 power (nothing else in the deck pumps).
//   - Code of Constraint: a permanent marked to skip its next untap step.
func TestMBCFilters5Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		reached func(ref *GorgeSide, o *state.Object) bool
	}{
		{
			name: "Addendum/Tail_Swipe",
			deck: []cardset.Entry{{Name: "Tail Swipe", Count: 14}, {Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 16}},
			reached: func(ref *GorgeSide, o *state.Object) bool {
				return o.Face() != nil && o.Face().Name == "Grizzly Bears" && ref.E.Power(o.ID) == 3
			},
		},
		{
			name: "Addendum/Code_of_Constraint",
			deck: []cardset.Entry{{Name: "Code of Constraint", Count: 16}, {Name: "Island", Count: 24}},
			reached: func(_ *GorgeSide, o *state.Object) bool {
				return o.CantUntapNextStep
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]cardset.Entry{tc.deck, bears}
			reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
			if err != nil {
				t.Fatal(err)
			}
			var names [2][]string
			var cs [2][]*cards.Card
			for s, d := range decks {
				for _, e := range d {
					names[s] = append(names[s], e.Name)
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for range e.Count {
						cs[s] = append(cs[s], c)
					}
				}
			}
			set := printedDelveSet(t, names)
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatal(err)
			}
			reached := 0
			for seed := uint64(1); seed <= 16; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"filters5-a", "filters5-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit := false
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					for p := range state.PlayerID(2) {
						for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
							if o := ref.E.G.Obj(id); o != nil && tc.reached(ref, o) {
								hit = true
							}
						}
					}
					return nil
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
				CloseSide(k)
				if err := unmodelledStallError(t.Name(), &res); err != nil {
					t.Errorf("seed %d: %v", seed, err)
				}
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				if hit {
					reached++
				}
			}
			t.Logf("%s: the Addendum applied in %d of 16 games", tc.name, reached)
			if reached < 3 {
				t.Errorf("the Addendum applied in %d of 16 games, want at least 3", reached)
			}
		})
	}
}
