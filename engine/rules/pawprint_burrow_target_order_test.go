package rules

// CR 700.2i's printed-order execution (the BLB Seasons) must not desynchronize
// the per-mode target bindings. A budgeted Charm records ChosenModes in
// printed order before the per-mode target ask, so charmTargetSlots,
// recheckCharmTargets and charmDistinctTargetRun all walk the same order.
// Season of the Burrow is the corpus carrier with two distinct target-bearing
// modes (exile a nonland permanent, return a small permanent card from your
// graveyard), so answering them in reverse printed order is the regression:
// pre-fix the target groups stayed in ANSWER order while resolution reordered
// only the mode names, and both modes silently no-opped.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestPawprintBurrowTargetOrder(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	burrow := lookup(t, reg, "Season of the Burrow")
	bear := lookup(t, reg, "Grizzly Bears")
	lion := lookup(t, reg, "Savannah Lions")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{burrow, bear, lion}, nil)

	burrowID := state.ObjID(0)
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == "Season of the Burrow" {
			burrowID = cand
		}
	}
	if burrowID == 0 {
		burrowID = moveByName(t, e, 0, "Season of the Burrow", state.ZHand)
	}
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	lionID := moveByName(t, e, 0, "Savannah Lions", state.ZBattlefield)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Grizzly Bears zone = %+v, want graveyard", o)
	}
	if o := e.G.Obj(lionID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Savannah Lions zone = %+v, want battlefield", o)
	}

	addMana(t, e, 0, "WWWWW") // 3 W W
	d := castFixture(t, e, burrowID, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	exile, reanimate := -1, -1
	for _, o := range d.Options {
		switch o.Value {
		case 2:
			exile = o.Index
		case 3:
			reanimate = o.Index
		}
	}
	if exile < 0 || reanimate < 0 {
		t.Fatalf("Season of the Burrow modes not priced 2/3: %+v", d.Options)
	}
	// Reverse printed order: DBReanimate ({P}{P}{P}) before DBExile ({P}{P}).
	submitChoices(t, e, reanimate, exile)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want the two modes' combined target ask", d)
	}
	lionIdx, bearIdx := -1, -1
	for _, o := range d.Options {
		switch o.Obj {
		case lionID:
			lionIdx = o.Index
		case bearID:
			bearIdx = o.Index
		}
	}
	if lionIdx < 0 || bearIdx < 0 {
		t.Fatalf("target ask does not offer both legal targets: lion %d bear %d in %+v", lionIdx, bearIdx, d.Options)
	}
	submitChoices(t, e, lionIdx, bearIdx)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(lionID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Savannah Lions zone = %+v, want exile (DBExile's target)", o)
	}
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Grizzly Bears zone = %+v, want battlefield (DBReanimate's target)", o)
	}
	if o := e.G.Obj(burrowID); o == nil || o.Zone != state.ZGraveyard {
		t.Errorf("Season of the Burrow resolved to %+v, want graveyard", o)
	}
	replayCheck(t, e, cfg)
}
