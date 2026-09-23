package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func damageCostLKIEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	sourceCard := card(t, "Name:DamageSource\nTypes:Artifact Creature\nPT:2/2\nOracle:x\n")
	e, _, _ := corpusDeckEngine(t, nil, []*cards.Card{sourceCard})
	source := e.G.Zone(state.ZBattlefield, 0)[0]
	if e.G.Obj(source).Zone != state.ZBattlefield {
		t.Fatal("damage source precondition: source is not on battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: source, From: state.ZBattlefield, To: state.ZExile})
	if e.G.Obj(source).Zone != state.ZExile {
		t.Fatalf("damage source precondition: source zone = %s, want exile", e.G.Obj(source).Zone)
	}
	if e.HasKeyword(source, "Infect") || e.HasKeyword(source, "Lifelink") {
		t.Fatal("damage source precondition: live keywords unexpectedly match the LKI")
	}
	return e, source
}

func assertDamageCostLKI(t *testing.T, e *Engine, source state.ObjID) {
	t.Helper()
	if e.G.Obj(source).Zone != state.ZExile {
		t.Fatalf("source zone = %s, want exile", e.G.Obj(source).Zone)
	}
	var damage *events.Event
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.Damage && ev.Player == 1 {
			damage = ev
		}
	}
	if damage == nil || damage.Counter != "infect" || damage.Amount != 4 {
		t.Fatalf("payer damage = %+v, want DamageYou<4> in infect form", damage)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("payer life = %d, want 20 (infect damage does not reduce life)", got)
	}
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("source controller life = %d, want 24 from lifelink LKI", got)
	}
}

func TestDamageYouCostUsesSourceDamageKeywordLKI(t *testing.T) {
	e, source := damageCostLKIEngine(t)
	lki := damageKeywordLKI{infect: true, lifelink: true}
	if e.HasKeyword(source, "Infect") == lki.infect || e.HasKeyword(source, "Lifelink") == lki.lifelink {
		t.Fatal("precondition: live source keywords must differ from the saved LKI")
	}
	e.payDamageCost(1, 4, source, lki, 0)
	assertDamageCostLKI(t, e, source)
}

func TestUnlessDamageCostUsesSourceDamageKeywordLKI(t *testing.T) {
	e, source := damageCostLKIEngine(t)
	lki := effects.DamageSourceLKI{Infect: true, Lifelink: true, Controller: 0}
	if e.HasKeyword(source, "Infect") == lki.Infect || e.HasKeyword(source, "Lifelink") == lki.Lifelink {
		t.Fatal("precondition: live source keywords must differ from the saved LKI")
	}
	ctx := &effects.Ctx{Source: source, Controller: 1,
		DamageSourceLKI: map[state.ObjID]effects.DamageSourceLKI{source: lki}}
	e.payUnlessDamageCost(ctx, 1, 4)
	assertDamageCostLKI(t, e, source)
}
