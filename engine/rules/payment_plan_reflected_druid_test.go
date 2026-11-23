package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// TestReflectedDruidCountsTappedLandsForCast: Incubation Druid's reflected
// ability reads what a land you control COULD produce (CR 106.7), tapped or
// not, and adds three of that type with a +1/+1 counter on it. Beside one
// TAPPED Forest the Druid alone pays a {G} spell; the planner must price the
// SVar Amount$ instead of deferring the source.
func TestReflectedDruidCountsTappedLandsForCast(t *testing.T) {
	t.Parallel()
	const spellSrc = "Name:Green Probe\nManaCost:G\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	e, _, spell := newFixtureDeck(t, 9931, spellSrc)
	toMain1(t, e)
	forest := onBoardCard(t, e, 0, corpusCard(t, "Forest"))
	druid := onBoardCard(t, e, 0, corpusCard(t, "Incubation Druid"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: druid, Counter: "P1P1", Amount: 1})
	e.G.Obj(druid).SummonSick = false
	e.G.Obj(forest).Tapped = true
	if o := e.G.Obj(druid); o.Tapped || o.Counter("P1P1") != 1 {
		t.Fatalf("precondition: druid tapped=%v counters=%v, want untapped with one P1P1", o.Tapped, o.Counter("P1P1"))
	}
	if ma := corpusCard(t, "Incubation Druid").Faces[0].Abilities[1]; !pay.PaymentPlanReflectedShape(ma) {
		t.Fatalf("precondition: the Druid's reflected ability is not a planner shape")
	}
	condSplitPayable(t, e, spell, "Incubation Druid beside a tapped Forest, {G} spell")
}
