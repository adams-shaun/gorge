package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestChangeZoneAttachedToFilterSkipsProtectedBearer is CR 303.4f with CR
// 702.16c: One Last Job's `AttachedTo$ Creature.YouCtrl` poses the controller's
// choice among the creatures the returning Aura can legally enchant, and a
// creature with protection from the Aura's colour is not one. Holy Strength is
// white and Black Knight has protection from white, so Grizzly Bears is the
// only legal bearer: no ask is posed (a lone answer is no choice) and the Aura
// enters attached to it. Offering the Knight would make this a two-option ask.
func TestChangeZoneAttachedToFilterSkipsProtectedBearer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, ids := attachedToFixture(t, reg, 9413, []string{"One Last Job"},
		[]string{"Grizzly Bears", "Black Knight"}, []string{"Holy Strength"})
	bears, knight, aura := ids["Grizzly Bears"], ids["Black Knight"], ids["Holy Strength"]
	if bears == 0 || knight == 0 || bears == knight || aura == 0 {
		t.Fatalf("precondition: two distinct battlefield creatures and the graveyard Aura, got %d, %d, %d",
			bears, knight, aura)
	}
	if o := e.G.Obj(knight); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Black Knight %d = %+v, want it on the battlefield", knight, o)
	}
	if o := e.G.Obj(aura); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Holy Strength %d = %+v, want it in the graveyard", aura, o)
	}
	castOneLastJobAttachMode(t, e, aura)
	// passUntilStackEmpty fatals on any unhandled KChoose: offering the
	// protected Knight beside the Bears fails here.
	passUntilStackEmpty(t, e, 30)
	o := e.G.Obj(aura)
	if o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bears {
		t.Fatalf("Holy Strength %+v, want it attached to Grizzly Bears %d, not the protected Black Knight %d",
			o, bears, knight)
	}
}
