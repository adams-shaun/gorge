package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Resplendent Marshal's "each other creature you control that shares a
// creature type with the exiled card": the Remembered the filter reads is the
// host's list (the exiled Briarbridge Tracker), never the Marshal the ETB
// trigger fired on, which sits in Remembered as the fire-time capture.
func TestSharesCreatureTypeWithRememberedSkipsTheTriggerCapture(t *testing.T) {
	g := state.NewGame(names(2))
	trk := g.AddObject(mkCard(t, "Name:Tracker\nTypes:Creature Human Scout\nPT:2/3\nOracle:x\n"), 0)
	other := g.AddObject(mkCard(t, "Name:Marshal\nTypes:Creature Angel Warrior\nPT:3/3\nOracle:x\n"), 0)
	cast := g.AddObject(mkCard(t, "Name:Marshal\nTypes:Creature Angel Warrior\nPT:3/3\nOracle:x\n"), 0)
	scout := g.AddObject(mkCard(t, "Name:Scout\nTypes:Creature Scout\nPT:1/1\nOracle:x\n"), 0)
	g.SetZone(state.ZExile, 0, []state.ObjID{trk.ID})
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{other.ID, cast.ID, scout.ID})
	const spec = "Creature.StrictlyOther+YouCtrl+sharesCreatureTypeWith Remembered"
	sc := SpecContext{You: 0, Source: cast.ID,
		Remembered: []state.Target{{Obj: cast.ID}, {Obj: trk.ID}},
		Captured:   []state.Target{{Obj: cast.ID}}}
	if !MatchesSpecCtx(g, spec, scout.ID, sc) {
		t.Fatalf("precondition: a Scout shares a creature type with the exiled Tracker")
	}
	if MatchesSpecCtx(g, spec, other.ID, sc) {
		t.Errorf("the other Marshal shares a type only with the capture (the Marshal that entered), not the exiled Tracker")
	}
}
