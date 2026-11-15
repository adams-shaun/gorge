package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCTriggerReferents11Lowers plays the cards of the MBC
// trigger-referents-11 family's spellings in lockstep against gorge on a
// table built for exactly their decks (kgen refused each before):
//
//   - LandPlayed ValidActivatingPlayer$ You (Shanid, Sleepers' Scourge:
//     "whenever you play a legendary land ... you draw a card and you lose 1
//     life"): the deck's only lands that are legendary are the three
//     channel lands and nothing else loses life, so a LifeChange of -1 for
//     seat 0 that is no Damage event's is the trigger's.
//   - Draw Defined$ You & TriggeredTarget (Xyris, the Writhing Storm: "you
//     and that player each draw that many cards"): the bears' owner draws
//     one card in each draw step and nothing else, so a Snake token on
//     Xyris's side is the opponent's extra draw (Xyris's first trigger).
//
// The fourth spelling, Permanent.nonLand+ControlledBy TriggeredTarget
// (O-Kagachi, Vengeful Kami), has no lockstep carrier: O-Kagachi stays refused
// on its Player.attackedYouTheirLastTurn filter, and the cards that share the
// shape (Throat Slitter, Lightwielder Paladin) are in no pool a table can be
// built from. Its seat-exact lowering is pinned in kgen's
// TestTrigReferents11Forms.
func TestMBCTriggerReferents11Lowers(t *testing.T) {
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
			name: "LandPlayed/Shanid",
			deck: []cardset.Entry{{Name: "Shanid, Sleepers' Scourge", Count: 4}, {Name: "Sokenzan, Crucible of Defiance", Count: 7},
				{Name: "Eiganjo, Seat of the Empire", Count: 7}, {Name: "Shizo, Death's Storehouse", Count: 7},
				{Name: "Mountain", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}},
			reached: func(ref *GorgeSide, next *int) bool {
				evs := ref.E.L.Events
				hit := false
				for i := max(*next, 1); i < len(evs); i++ {
					e := evs[i]
					hit = hit || (e.Kind == events.LifeChange && e.Player == 0 && e.Amount == -1 && evs[i-1].Kind != events.Damage)
				}
				*next = len(evs)
				return hit
			},
		},
		{
			name: "YouAndTriggeredTarget/Xyris",
			deck: []cardset.Entry{{Name: "Xyris, the Writhing Storm", Count: 14}, {Name: "Forest", Count: 9}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 8}},
			reached: func(ref *GorgeSide, _ *int) bool {
				return trigRefsNamedOn(ref, 0, "Snake Token") > 0
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
				m, err := NewMatchup(reg, [2]string{"trigrefs11-a", "trigrefs11-b"}, cs, 0)
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
