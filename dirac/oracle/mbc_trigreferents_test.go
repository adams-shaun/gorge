package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCTriggerReferents2Lowers plays the cards of the MBC
// trigger-referents-2 family's spellings in lockstep against gorge on a
// table built for exactly their decks (kgen refused every one of them
// before):
//
//   - TriggeredTarget$CardsInHand, TriggeredTarget$Valid Land.YouCtrl,
//     PlayerCountDefinedNonTriggeredTarget$Highest{CardsInHand, Valid
//     Land.YouCtrl, LifeTotal} (Battle Angels of Tyr): the angel connects
//     and its three "more than each other player" gates are read. Nothing
//     else in either deck makes a Treasure or gains life, so a Treasure on
//     the angels' side is the land gate passed and a rise in their life the
//     life gate.
//   - TriggeredTarget$Valid Permanent.nonDemon+YouCtrl (Rakdos the
//     Defiler's "that player sacrifices half the non-Demon permanents they
//     control"): nothing else takes a Forest off the bears' battlefield, so
//     one fewer is the damage trigger's sacrifice.
func TestMBCTriggerReferents2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		reached func(ref *GorgeSide, prev *int) bool
	}{
		{
			name: "NonTriggeredTarget/Battle_Angels_of_Tyr",
			deck: []cardset.Entry{{Name: "Battle Angels of Tyr", Count: 16}, {Name: "Plains", Count: 24}},
			reached: func(ref *GorgeSide, prev *int) bool {
				life := int(ref.E.G.Players[0].Life)
				hit := *prev >= 0 && life > *prev
				*prev = life
				return hit || trigRefsNamedOn(ref, 0, "Treasure") > 0
			},
		},
		{
			name: "TriggeredTargetValid/Rakdos_the_Defiler",
			deck: []cardset.Entry{{Name: "Rakdos the Defiler", Count: 14}, {Name: "Swamp", Count: 13}, {Name: "Mountain", Count: 13}},
			reached: func(ref *GorgeSide, prev *int) bool {
				n := trigRefsNamedOn(ref, 1, "Forest")
				hit := *prev >= 0 && n < *prev
				*prev = n
				return hit
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
				m, err := NewMatchup(reg, [2]string{"trigrefs-a", "trigrefs-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit, prev := false, -1
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					hit = tc.reached(ref, &prev) || hit
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
			t.Logf("%s: the mechanism reached in %d of 16 games", tc.name, reached)
			if reached < 3 {
				t.Errorf("the mechanism reached in %d of 16 games, want at least 3", reached)
			}
		})
	}
}

// trigRefsNamedOn counts the permanents named name on seat p's battlefield.
func trigRefsNamedOn(ref *GorgeSide, p state.PlayerID, name string) int {
	n := 0
	for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
		if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}
