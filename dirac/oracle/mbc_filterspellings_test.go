package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFilters3Lowers plays one lowered card of each spelling the MBC
// filters-3 family lowers, in lockstep against gorge on a table built for
// exactly its decks (kgen refused every one of these cards before):
//
//   - with<Two-word keyword> (Thunderous Orator's ConditionPresent$
//     Creature.YouCtrl+withFirst Strike / +withDouble Strike): the Orator
//     attacks beside a Youthful Knight or a Fencing Ace and gains the keyword.
//   - <Colour>Source (Jaya's Card.RedSource+Other+YouCtrl, Torbran's
//     Card.RedSource+YouCtrl with ValidTarget$ Player.Opponent,
//     Permanent.OppCtrl): a red source deals the replaced damage. Nothing in
//     those decks deals 3 (Jaya) or 4 (Torbran) damage without the
//     replacement, so a Damage event of that amount is the replacement
//     applied.
func TestMBCFilters3Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		reached func(ref *GorgeSide) bool
		atEnd   bool // reached reads the event log once, at the game's end
	}{
		{
			name: "withFirst_Strike/Thunderous_Orator",
			deck: []cardset.Entry{{Name: "Thunderous Orator", Count: 12}, {Name: "Youthful Knight", Count: 6}, {Name: "Fencing Ace", Count: 6}, {Name: "Plains", Count: 16}},
			reached: func(ref *GorgeSide) bool {
				for p := range state.PlayerID(2) {
					for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
						if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Thunderous Orator" &&
							(ref.E.HasKeyword(id, "First Strike") || ref.E.HasKeyword(id, "Double Strike")) {
							return true
						}
					}
				}
				return false
			},
		},
		{
			name:    "RedSource/Jaya",
			deck:    []cardset.Entry{{Name: "Jaya, Venerated Firemage", Count: 10}, {Name: "Shock", Count: 14}, {Name: "Mountain", Count: 16}},
			reached: damageOf(3),
			atEnd:   true,
		},
		{
			name:    "RedSource/Torbran",
			deck:    []cardset.Entry{{Name: "Torbran, Thane of Red Fell", Count: 10}, {Name: "Shock", Count: 14}, {Name: "Mountain", Count: 16}},
			reached: damageOf(4),
			atEnd:   true,
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
				m, err := NewMatchup(reg, [2]string{"filters3-a", "filters3-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit := false
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				if tc.atEnd {
					rc.Finish = func(ref *GorgeSide) { hit = tc.reached(ref) }
				} else {
					rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
						hit = hit || tc.reached(ref)
						return nil
					}
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
			t.Logf("%s: the mechanism reached in %d of 16 games", tc.name, reached)
			if reached < 3 {
				t.Errorf("the mechanism reached in %d of 16 games, want at least 3", reached)
			}
		})
	}
}

// damageOf: some Damage event dealt exactly n.
func damageOf(n int32) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		return slices.ContainsFunc(ref.E.L.Events, func(e events.Event) bool { return e.Kind == events.Damage && e.Amount == n })
	}
}
