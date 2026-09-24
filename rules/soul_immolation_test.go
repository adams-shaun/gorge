package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Soul Immolation's RaiseCost must retain Blight<X> through cost composition:
// X=2 is announced, paid on a controlled creature, and used by DamageAll.
func TestSoulImmolationRaiseCostBlightX(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := blightEngine(t, reg, 2, "Soul Immolation")
	spell := blightMove(t, e, 0, "Soul Immolation", state.ZHand)
	own := blightMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	blightMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	opponent := blightMove(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	// Preconditions: the creature the blight payment will pick from the
	// caster's side is a real 2-toughness battlefield creature (so the X cap
	// from `XMax$ GrTo` is 2 and the ask is non-vacuous), and the opposing
	// creature is a 2/2 on the battlefield, so X=2 damage is lethal and
	// distinguishable from X=0.
	if o := e.G.Obj(own); o == nil || o.Zone != state.ZBattlefield || e.Toughness(own) != 2 {
		t.Fatalf("precondition: own creature %d must be a 2-toughness battlefield creature", own)
	}
	if o := e.G.Obj(opponent); o == nil || o.Zone != state.ZBattlefield || e.Toughness(opponent) != 2 {
		t.Fatalf("precondition: opposing creature %d must be a 2-toughness battlefield creature", opponent)
	}
	passUntilStackEmpty(t, e, 20)
	passToSeat(t, e, 0)
	addMana(t, e, 0, "RRCCC")
	beforeLife := lifeOf(t, e, 1)
	idx := -1
	for _, o := range castOptions(t, e) {
		if o.Obj == spell {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Soul Immolation not castable: %+v", castOptions(t, e))
	}
	submitChoices(t, e, idx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("want X announcement, got %+v; cast=%+v; events=%+v", d, e.cast, e.L.Events[len(e.L.Events)-8:])
	}
	xIdx := -1
	for _, o := range d.Options {
		if o.Amount == 2 {
			xIdx = o.Index
		}
	}
	if xIdx < 0 {
		t.Fatalf("X=2 not offered: %+v", d.Options)
	}
	submitChoices(t, e, xIdx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "blightcost" {
		t.Fatalf("want Blight<X> cost decision, got %+v", d)
	}
	chosen := d.Options[0].Obj
	if chosen != own {
		t.Fatalf("blight target %d, want controlled creature %d", chosen, own)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 30)
	if got := blightCounters(t, e)[own]; got != 2 {
		t.Fatalf("paid Blight<X=2> counters=%d, want 2", got)
	}
	passed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == opponent && ev.Amount == 2 {
			passed = true
		}
	}
	if !passed {
		t.Fatalf("no Damage event of 2 on opposing creature %d; want X=2 damage", opponent)
	}
	if o := e.G.Obj(opponent); o != nil && o.Zone == state.ZBattlefield {
		t.Fatalf("opposing 2/2 %d survived 2 damage (zone=%v, marked damage=%d)", opponent, o.Zone, o.Damage)
	}
	if got := lifeOf(t, e, 1); got != beforeLife-2 {
		t.Fatalf("opponent life=%d, want %d after X=2 damage", got, beforeLife-2)
	}
}
