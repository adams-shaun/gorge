package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggerCostPayLockstep plays one card of each trigger whose Execute is an
// `AB$` with a `Cost$` (CR 603.12 reflexive "you may pay {N}. If you do, ..." and
// the mandatory `Sac<1/CARDNAME>` body of Promise of Bunrei), in lockstep
// against gorge on a table built for exactly its decks. The paying seat's
// "trigger_cost_pay" ask (or the mandatory sacrifice that makes the body
// resolve) must be posed identically on both sides.
func TestTriggerCostPayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	askReached := func(ref *GorgeSide) bool {
		d := ref.Pending()
		if d == nil {
			return false
		}
		for _, o := range d.Options {
			if o.Kind == "trigger_cost_pay" {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		name    string
		deck    []cardset.Entry
		opp     []cardset.Entry // the other seat's deck, bears when nil
		heur    bool            // gorge's heuristic bot plays both seats (combat, so a creature dies)
		reached func(ref *GorgeSide) bool
	}{
		{
			// the mandatory Sac<1/CARDNAME> cost, settled unasked: the Bunrei is sacrificed and four Spirit tokens follow
			name: "Promise_of_Bunrei",
			heur: true,
			opp:  []cardset.Entry{{Name: "Hill Giant", Count: 16}, {Name: "Lightning Bolt", Count: 8}, {Name: "Mountain", Count: 16}},
			deck: []cardset.Entry{{Name: "Promise of Bunrei", Count: 8}, {Name: "Savannah Lions", Count: 16}, {Name: "Plains", Count: 16}},
			reached: func(ref *GorgeSide) bool {
				for p := range state.PlayerID(2) {
					for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
						if o := ref.E.G.Obj(id); o != nil && o.IsToken {
							return true
						}
					}
				}
				return false
			},
		},
		{
			name:    "Mentor_of_the_Meek",
			deck:    []cardset.Entry{{Name: "Mentor of the Meek", Count: 8}, {Name: "Grizzly Bears", Count: 16}, {Name: "Plains", Count: 16}},
			reached: askReached,
		},
		{
			// a depletion land (Sandstone Needle, {T} + a counter for {R}{R}) funds the trigger's {1}
			// only through gorge's potential-mana bound and its manual window
			name: "Goblinslide_depletion_land",
			deck: []cardset.Entry{{Name: "Goblinslide", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Sandstone Needle", Count: 24}},
			reached: func(ref *GorgeSide) bool {
				for p := range state.PlayerID(2) {
					for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
						if o := ref.E.G.Obj(id); o != nil && o.IsToken {
							return true
						}
					}
				}
				return false
			},
		},
		{
			name:    "Jubilant_Mascot_depletion_land",
			deck:    []cardset.Entry{{Name: "Jubilant Mascot", Count: 10}, {Name: "Sandstone Needle", Count: 10}, {Name: "Plains", Count: 20}},
			reached: askReached,
		},
		{
			name:    "Masked_Admirers",
			deck:    []cardset.Entry{{Name: "Masked Admirers", Count: 10}, {Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 20}},
			reached: askReached,
		},
		{
			name:    "Lilianas_Devotee",
			deck:    []cardset.Entry{{Name: "Liliana's Devotee", Count: 10}, {Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 20}},
			reached: askReached,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]cardset.Entry{tc.deck, bears}
			if tc.opp != nil {
				decks[1] = tc.opp
			}
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
				m, err := NewMatchup(reg, [2]string{"trigcost-a", "trigcost-b"}, cs, 0)
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
					hit = hit || tc.reached(ref)
					return nil
				}
				var pol Policy = RandomPolicy(seed)
				if tc.heur {
					pol = HeuristicPolicy(seed)
				}
				res := Lockstep(m, seed, k, pol, rc)
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
