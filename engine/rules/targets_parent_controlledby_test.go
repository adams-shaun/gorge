package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSearingBlazeCreatureLinkReadsParentTarget pins the `ControlledBy
// ParentTarget` / `ControlledBy ParentTargetedController` spellings on the
// corpus Searing Blaze's SearingDamage link: a player parent restricts the
// creature to that player's, an object parent to its controller's.
func TestSearingBlazeCreatureLinkReadsParentTarget(t *testing.T) {
	reg := freshCorpusRegistry(t, "s/searing_blaze.txt")
	blaze := mustCorpusCard(t, reg, "Searing Blaze")
	if len(blaze.Faces) == 0 || len(blaze.Faces[0].Abilities) == 0 || blaze.Faces[0].Abilities[0].Sub == nil {
		t.Fatal("precondition: Searing Blaze has no sub-ability")
	}
	sub := blaze.Faces[0].Abilities[0].Sub
	if sub.API != "DealDamage" || sub.Params["ValidTgts"] != "Creature.ControlledBy ParentTargetedController,Creature.ControlledBy ParentTarget" {
		t.Fatalf("precondition: SearingDamage = %+v", sub)
	}

	e := newSeats(t, 3)
	source := e.G.AddObject(blaze, 0).ID
	var bears [3]state.ObjID
	for s := range bears {
		bears[s] = onBoardCard(t, e, state.PlayerID(s), card(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"))
		if o := e.G.Obj(bears[s]); o == nil || o.Zone != state.ZBattlefield || o.Controller != state.PlayerID(s) {
			t.Fatalf("precondition: bear %d = %+v", s, o)
		}
	}

	for _, tc := range []struct {
		name   string
		parent state.Target
		want   state.ObjID
	}{
		{"player seat 1", state.Target{IsPlayer: true, Player: 1}, bears[1]},
		{"player seat 2", state.Target{IsPlayer: true, Player: 2}, bears[2]},
		{"creature of seat 2", state.Target{Obj: bears[2]}, bears[2]},
		{"creature of seat 1", state.Target{Obj: bears[1]}, bears[1]},
	} {
		got := e.LegalSubTargets(0, source, sub, []state.Target{tc.parent})
		if len(got) != 1 || got[0].Obj != tc.want {
			t.Errorf("%s: offer = %+v; want only %d", tc.name, got, tc.want)
		}
	}
}
