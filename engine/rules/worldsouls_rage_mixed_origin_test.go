package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestWorldsoulsRagePicksFromHandAndGraveyardInOneAnswer pins the multi-pick
// mixed-Hand origin (Worldsoul's Rage's DBChangeZone with a literal count,
// CR 400.1 "from your hand and/or graveyard"): ONE search offers the land in
// hand and the land in the graveyard with Max 2, nothing moves before the
// answer, both answered lands enter tapped, and no mixed-origin Note fires.
func TestWorldsoulsRagePicksFromHandAndGraveyardInOneAnswer(t *testing.T) {
	t.Parallel()
	e, cfg, id := kr1New(t, 314, kr1Sorcery("WorldsoulFx",
		"A:SP$ ChangeZone | Origin$ Hand,Graveyard | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 2 | Tapped$ True"),
		[]string{kr1Isle, kr1Isle}, nil)
	addMana(t, e, 0, "B")
	kr1ClearHand(t, e, 0, id)
	handLand := kr1Put(t, e, 0, "Isle", state.ZHand)
	graveLand := kr1Put(t, e, 0, "Isle", state.ZGraveyard)
	d := kr1Cast(t, e, id)
	if d == nil || d.ResumeKind != "search" || d.Max != 2 {
		t.Fatalf("mixed-origin search = %+v, want one search with Max 2", d)
	}
	hi := kr1OptIndex(t, d, handLand)
	gi := kr1OptIndex(t, d, graveLand)
	kr1Want(t, e, state.ZHand, handLand)
	kr1Want(t, e, state.ZGraveyard, graveLand)
	if d = kr1Pick(t, e, hi, gi); d != nil && d.ResumeKind == "search" {
		t.Fatalf("a second search was posed: %+v", d)
	}
	for _, land := range []state.ObjID{handLand, graveLand} {
		kr1Want(t, e, state.ZBattlefield, land)
		if !e.G.Obj(land).Tapped {
			t.Fatalf("land %d entered untapped", land)
		}
	}
	if kr1HasNote(e, "mixed ChangeZone Origin") {
		t.Fatal("a mixed-origin Note fired although both chosen lands moved")
	}
	replayCheck(t, e, cfg)
}
