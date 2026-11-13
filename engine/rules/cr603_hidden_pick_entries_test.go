package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR603_2cLivelyDirgeHiddenReturnIsOneEntryEvent: a Hidden$ pick from a
// public origin that returns several creature cards puts them onto the
// battlefield as ONE event (CR 603.2c, 603.6a), so Satoru, the Infiltrator's
// "whenever Satoru and/or one or more other nontoken creatures you control
// enter" (Mode$ ChangesZoneAll) triggers once for both cards, not once each.
func TestCR603_2cLivelyDirgeHiddenReturnIsOneEntryEvent(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Lively Dirge", "Satoru, the Infiltrator", "Grizzly Bears", "Savannah Lions")
	satoru := searchMoveByName(t, e, "Satoru, the Infiltrator", state.ZBattlefield)
	// Satoru's own entry triggers it; drain that trigger before the cast so
	// the hand count below measures only the Lively Dirge entries.
	passUntilResolved(t, e, 40)
	bears := searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	lions := searchMoveByName(t, e, "Savannah Lions", state.ZGraveyard)
	if bears == lions {
		t.Fatalf("precondition: one creature moved twice")
	}
	if o := e.G.Obj(satoru); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Satoru not on the battlefield: %+v", o)
	}
	for _, id := range []state.ObjID{bears, lions} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: creature %d not in the graveyard: %+v", id, o)
		}
	}

	id := searchMoveByName(t, e, "Lively Dirge", state.ZHand)
	addMana(t, e, 0, "BBBBBB")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("Lively Dirge seq %d: pending after cast = %+v, want the Spree mode ask", len(e.L.Events), d)
	}
	submitChoices(t, e, modeOptionContaining(t, d, "Return up to two"))
	d = passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "hidden_pick" {
		t.Fatalf("Lively Dirge seq %d: pending = %+v, want the hidden pick", len(e.L.Events), d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("Lively Dirge seq %d: hidden pick = %d..%d, want 0..2", len(e.L.Events), d.Min, d.Max)
	}
	pick := make([]int, 0, 2)
	for _, o := range d.Options {
		if o.Obj == bears || o.Obj == lions {
			pick = append(pick, o.Index)
		}
	}
	if len(pick) != 2 {
		t.Fatalf("Lively Dirge seq %d: hidden pick options = %+v, want both graveyard creatures", len(e.L.Events), d.Options)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, pick...)

	d = passUntilResolved(t, e, 40)
	for i := 0; i < 40; i++ {
		if d == nil {
			t.Fatalf("Lively Dirge seq %d: no decision pending", len(e.L.Events))
		}
		switch {
		case d.Kind == decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		case d.Kind == decision.KPriority && stackEmpty(e) && len(e.pendingTriggers) == 0:
			i = 40
			continue
		case d.Kind != decision.KPriority:
			t.Fatalf("Lively Dirge seq %d: unexpected %s decision %+v", len(e.L.Events), d.Kind, d)
		}
		d = passUntilResolved(t, e, 40)
	}

	for _, id := range []state.ObjID{bears, lions} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Lively Dirge seq %d: creature %d not returned to the battlefield: %+v", len(e.L.Events), id, o)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Errorf("Lively Dirge seq %d: hand delta %d, want 1 (Satoru triggers once for the one entry event)", len(e.L.Events), got-handBefore)
	}
	replayCheck(t, e, cfg)
}
