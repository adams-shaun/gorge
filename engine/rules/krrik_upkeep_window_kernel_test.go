package rules

// K'rrik's cumulative-upkeep mana window, driven through the resolution kernel
// (engine/rules/resolve).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestKrrikCumulativeUpkeepPosesManaWindowWhenSourceCanPay(t *testing.T) {
	t.Parallel()
	e, cfg, bear, swamp := krrikUpkeepWindowEngine(t, krrikCumulativeFixture)
	krrikUpkeepPreconditions(t, e, bear, swamp, "Cumulative upkeep")
	if c := ParseCost("B"); c.Colored[state.MB] != 1 || c.Generic != 0 || !c.Priceable() {
		t.Fatalf("precondition: ParseCost(B) = %+v, want one plain {B} pip", c)
	}

	resolveCumulativeUpkeep(t, e)

	// The window must be posed: the pool alone cannot pay the {B} pip, but
	// the untapped Swamp can.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != bear {
		t.Fatalf("the cumulative-upkeep election must be posed, got %+v", d)
	}
	d = krrikElectManual(t, e, d, "cumulative")
	swampOpt := krrikFindWindowOption(t, d, "activate", swamp)
	krrikFindWindowOption(t, d, "done", 0)
	submitChoices(t, e, swampOpt) // tap the Swamp: the elected pay settles

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("paid the upkeep but the permanent is %+v", o)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life=%d, want 20 (the pip was paid with the Swamp's mana, not K'rrik's life)", life)
	}
	if !e.G.Obj(swamp).Tapped {
		t.Fatal("the Swamp must be tapped to pay the pip")
	}
	replayCheck(t, e, cfg)
}

func TestKrrikCumulativeUpkeepWindowDonePaysLife(t *testing.T) {
	t.Parallel()
	e, cfg, bear, swamp := krrikUpkeepWindowEngine(t, krrikCumulativeFixture)
	krrikUpkeepPreconditions(t, e, bear, swamp, "Cumulative upkeep")

	resolveCumulativeUpkeep(t, e)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != bear {
		t.Fatalf("the cumulative-upkeep election must be posed, got %+v", d)
	}
	d = krrikElectManual(t, e, d, "cumulative")
	doneOpt := krrikFindWindowOption(t, d, "done", 0)
	// The payment keeps the grant, so "done" settles the elected pay with
	// the 2 granted life.
	submitChoices(t, e, doneOpt)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("paid the upkeep with life but the permanent is %+v", o)
	}
	if life := e.G.Players[0].Life; life != 18 {
		t.Fatalf("life=%d, want 18 (\"done\" spends the {B} pip's 2 granted life)", life)
	}
	if e.G.Obj(swamp).Tapped {
		t.Fatal("the Swamp must stay untapped when the payer answers \"done\"")
	}
	replayCheck(t, e, cfg)
}
