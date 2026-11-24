package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMonstrosityXZeroBecomesMonstrous: CR 701.37a has no N > 0 condition.
// Hydra Broodmaster activated with X = 0 places no counters but becomes
// monstrous, its BecomeMonstrous trigger goes on the stack and resolves
// creating zero tokens, and the once-only ability is no longer offered.
func TestMonstrosityXZeroBecomesMonstrous(t *testing.T) {
	t.Parallel()
	e, cfg, id := monstrosityEngine(t, "Hydra Broodmaster", "GG")
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Monstrous {
		t.Fatalf("precondition: source zone=%v monstrous=%v", o.Zone, o.Monstrous)
	}
	idx := monstrosityAbilityIndex(t, e, id)
	opt, ok := findAbilityOption(e, id, idx)
	if !ok {
		t.Fatal("precondition: Hydra Broodmaster's Monstrosity X ability is not offered")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want X announcement, got %+v", d)
	}
	x0 := -1
	for _, o := range d.Options {
		if o.Kind == "x" && o.Amount == 0 {
			x0 = o.Index
		}
	}
	if x0 < 0 {
		t.Fatalf("X=0 not offered: %+v", d.Options)
	}
	submitChoices(t, e, x0)
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if !o.Monstrous || o.Counter("P1P1") != 0 {
		t.Fatalf("Hydra Broodmaster after X=0: monstrous=%v counters=%d, want monstrous with no counters", o.Monstrous, o.Counter("P1P1"))
	}
	if marks := monstrousMarkEvents(e); len(marks) != 1 || marks[0].Obj != id || marks[0].Amount != 0 {
		t.Fatalf("want one Monstrous mark carrying X=0, got %+v", marks)
	}
	pushed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == id {
			pushed = true
		}
	}
	if !pushed {
		t.Fatal("Hydra Broodmaster's BecomeMonstrous trigger was not pushed")
	}
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if t2 := e.G.Obj(oid); t2 != nil && t2.IsToken {
			t.Fatalf("X=0 trigger created a token %+v", t2.Face())
		}
	}
	// The spare G still floats, so {X}{X}{G} at X=0 stays affordable: only
	// the once-only monstrous gate can withhold the offer.
	if got := e.G.Players[0].Pool.Total(); got < 1 {
		t.Fatalf("precondition: spare mana gone (pool=%d), the offer check would be vacuous", got)
	}
	if _, ok := findAbilityOption(e, id, idx); ok {
		t.Fatal("monstrosity ability still offered after Hydra Broodmaster became monstrous")
	}
	replayCheck(t, e, cfg)
}
