package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestRemoveAnyCounterCostOffersCounterKindChoice pins the wildcard cost on a
// Fain-shaped activation: both the permanent and the counter kind are legal
// choices, and the selected kind is the one removed by payment.
func TestRemoveAnyCounterCostOffersCounterKindChoice(t *testing.T) {
	const fain = "Name:Fain, the Broker\nManaCost:2 B\nTypes:Legendary Creature Human Warlock\nPT:3/3\nK:Haste\n" +
		"A:AB$ Token | Cost$ T RemoveAnyCounter<1/Any/Creature> | TokenScript$ c_a_treasure_sac | SpellDescription$ Create a Treasure token.\nOracle:x\n"
	targetCard := "Name:Counter Creature\nTypes:Creature\nPT:2/2\nOracle:x\n"
	e, cfg, source := newFixtureDeck(t, 61, fain, targetCard)
	target := moveSeeded(t, e, 0, targetCard, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "CHARGE", Amount: 1})
	if e.G.Obj(target).Counter("P1P1") != 1 || e.G.Obj(target).Counter("CHARGE") != 1 {
		t.Fatal("setup: target must have both counters on the battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: source, From: e.G.Obj(source).Zone, To: state.ZBattlefield})
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("source setup: %+v", o)
	}
	opt := abilityOption(t, e, source, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("counter-kind decision = %+v, want two choices", d)
	}
	chosen := -1
	for _, o := range d.Options {
		if o.Obj == target && o.Counter == "CHARGE" {
			chosen = o.Index
		}
	}
	if chosen < 0 {
		t.Fatalf("CHARGE choice missing: %+v", d.Options)
	}
	submitChoices(t, e, chosen)
	if got := e.G.Obj(target).Counter("CHARGE"); got != 0 {
		t.Fatalf("selected CHARGE counter = %d, want 0", got)
	}
	if got := e.G.Obj(target).Counter("P1P1"); got != 1 {
		t.Fatalf("unselected +1/+1 counter = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestParseRemoveAnyCounterCost(t *testing.T) {
	c := ParseCost("T RemoveAnyCounter<1/Any/Creature>")
	if len(c.Unknown) != 0 || len(c.SubCounter) != 1 {
		t.Fatalf("parsed cost = %+v, want one modelled counter-removal part", c)
	}
	p := c.SubCounter[0]
	if p.N != 1 || p.Spec != "Any" || p.Target != "Creature" {
		t.Fatalf("counter-removal part = %+v", p)
	}
}
