package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCTriggerReferents7Lowers plays the cards the trigger-referents-7
// lowering family unblocked in lockstep against gorge, on a table built from
// the printed cards (printedDelveSet: kgen refuses a card it cannot lower, so
// without the lowering the table has no row and the matchup does not build):
//
//   - Standstill: Defined$ TriggeredCardOpponent, each opponent of the
//     spell's caster draws three after Standstill sacrifices itself. The
//     game must reach a sacrificed Standstill (it is in a graveyard).
//   - Apprentice Sorcerer: ActivationPhases$ Upkeep->BeginCombat with
//     ActivationFirstCombat$ True and PlayerTurn$ True. The game must reach a
//     tapped Sorcerer (the ping was activated), and lockstep compares the
//     offers at every decision, so an activation offered after the first
//     combat's start (or on the opponent's turn) diverges.
func TestMBCTriggerReferents7Lowers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		deck  []cardset.Entry
		saw   func(g *state.Game) bool
		seeds uint64
	}{
		{"Standstill", []cardset.Entry{{Name: "Standstill", Count: 8}, {Name: "Merfolk of the Pearl Trident", Count: 8}, {Name: "Island", Count: 24}},
			func(g *state.Game) bool { return zoneHas(g, state.ZGraveyard, "Standstill") }, 80},
		{"Apprentice Sorcerer", []cardset.Entry{{Name: "Apprentice Sorcerer", Count: 14}, {Name: "Island", Count: 26}},
			func(g *state.Game) bool {
				for p := range g.Players {
					for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
						if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Apprentice Sorcerer" && o.Tapped {
							return true
						}
					}
				}
				return false
			}, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := synthReg(t)
			decks := [2][]cardset.Entry{tc.deck, tc.deck}
			var names []string
			var cs [2][]*cards.Card
			for _, e := range tc.deck {
				names = append(names, e.Name)
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					cs[0] = append(cs[0], c)
					cs[1] = append(cs[1], c)
				}
			}
			set := printedDelveSet(t, [2][]string{names, names})
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("the card does not lower: %v", err)
			}
			reached := 0
			for seed := uint64(1); seed <= tc.seeds; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"mbc7-a", "mbc7-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				saw := false
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
					if !saw && tc.saw(ref.E.G) {
						saw = true
					}
					return nil
				}
				res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, map[string]int{}), rc)
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				if saw {
					reached++
				}
			}
			if reached == 0 {
				t.Fatalf("no seed of %d reached the card's effect", tc.seeds)
			}
			t.Logf("%d of %d seeds reached the effect", reached, tc.seeds)
		})
	}
}

// zoneHas reports whether any player's zone z holds a card named name.
func zoneHas(g *state.Game, z state.Zone, name string) bool {
	for p := range g.Players {
		for _, id := range g.Zone(z, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
	}
	return false
}
