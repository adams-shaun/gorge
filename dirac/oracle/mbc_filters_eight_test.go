package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFilters8Lowers plays one lowered card of each spelling the MBC
// filters-8 family lowers, in lockstep against gorge on a table built for
// exactly its decks (kgen refused both cards before):
//
//   - filter head TargetedCard (Echoing Truth's ChangeZoneAll ChangeType$
//     "TargetedCard.Self,Permanent.NotDefinedTargeted+sharesNameWith
//     Targeted"): against a deck of Grizzly Bears one Echoing Truth returns
//     the target and another Bears, two battlefield-to-hand moves in one
//     resolution.
//   - YourTeamCtrl (Combo Attack's ValidTgts$ Creature.YourTeamCtrl, read as
//     YouCtrl: gorge models no teams): Combo Attack resolves and its two
//     creatures deal damage.
//
// prop:IsRenowned / !IsRenowned stay refused: every card reading them needs
// the Renown keyword or another refused keyword (see the report).
func TestMBCFilters8Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	for _, tc := range []struct {
		name  string
		deck  []cardset.Entry
		spell string
		// reached: the resolution of spell (its Stack -> Graveyard move)
		// saw events matching ev on at least n distinct objects since the
		// spell went on the stack
		n  int
		ev func(e events.Event) bool
	}{
		{
			name:  "TargetedCard/Echoing_Truth",
			deck:  []cardset.Entry{{Name: "Echoing Truth", Count: 16}, {Name: "Island", Count: 24}},
			spell: "Echoing Truth",
			n:     2,
			ev: func(e events.Event) bool {
				return e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand
			},
		},
		{
			name:  "YourTeamCtrl/Combo_Attack",
			deck:  []cardset.Entry{{Name: "Combo Attack", Count: 12}, {Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 16}},
			spell: "Combo Attack",
			n:     1,
			ev:    func(e events.Event) bool { return e.Kind == events.Damage && e.Amount > 0 },
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
				m, err := NewMatchup(reg, [2]string{"filters8-a", "filters8-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				hit := false
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Finish = func(ref *GorgeSide) { hit = resolvedWith(ref, tc.spell, tc.n, tc.ev) }
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

// resolvedWith reports whether some spell named name left the stack for the
// graveyard after events matching ev on at least n distinct objects since it
// went on the stack.
func resolvedWith(ref *GorgeSide, name string, n int, ev func(events.Event) bool) bool {
	named := func(id state.ObjID) bool {
		o := ref.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == name
	}
	var seen map[state.ObjID]bool // objects of matching events since the spell went on the stack; nil off it
	for _, e := range ref.E.L.Events {
		switch {
		case (e.Kind == events.PutOnStack || e.Kind == events.MoveZone && e.To == state.ZStack) && named(e.Obj):
			seen = map[state.ObjID]bool{}
		case e.Kind == events.MoveZone && e.From == state.ZStack && named(e.Obj):
			if len(seen) >= n && e.To == state.ZGraveyard {
				return true
			}
			seen = nil
		case seen != nil && ev(e):
			seen[e.Obj] = true
		}
	}
	return false
}
