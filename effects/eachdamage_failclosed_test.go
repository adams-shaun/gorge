package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func eachDamageHasNote(h *fakeHost) bool {
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			return true
		}
	}
	return false
}

func TestEachDamageUnknownDamagerPredicateIsLoudAndFailsClosed(t *testing.T) {
	h := newHost(t, 2)
	src := librarySource(t, h)
	damager := putCreature(t, h, 0, "Damager", "3/3", "")
	if o := h.g.Obj(damager); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: damager must be on the battlefield")
	}
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t,
		"DB$ EachDamage | DefinedDamagers$ Valid Creature.Wumpus | Defined$ Self | NumDmg$ 1"))
	if got := damageEvents(h); len(got) != 0 {
		t.Fatalf("damage events = %+v, want none for unknown predicate", got)
	}
	if !eachDamageHasNote(h) {
		t.Fatalf("log = %+v, want a replay-visible Note for the unknown predicate", h.log)
	}
}

func TestEachDamageUnresolvableAmountIsLoudAndZero(t *testing.T) {
	h := newHost(t, 2)
	damager := putCreature(t, h, 0, "Damager", "3/3", "")
	target := putCreature(t, h, 1, "Target", "1/5", "")
	if h.g.Obj(damager).Zone != state.ZBattlefield || h.g.Obj(target).Zone != state.ZBattlefield {
		t.Fatal("precondition: damager and recipient must be on the battlefield")
	}
	Resolve(h, &Ctx{Source: damager, Controller: 0, Targets: []state.Target{{Obj: target}}}, sa(t,
		"DB$ EachDamage | ValidCards$ Creature.YouCtrl | Defined$ Targeted | NumDmg$ NoSuchSVar"))
	got := damageEvents(h)
	if len(got) != 1 || got[0].Amount != 0 {
		t.Fatalf("damage events = %+v, want one zero-amount event for unresolved NumDmg$", got)
	}
	if !eachDamageHasNote(h) {
		t.Fatalf("log = %+v, want a replay-visible Note for unresolved NumDmg$", h.log)
	}
}

func TestEachDamageDamagersCanFilterByTargetedController(t *testing.T) {
	h := newHost(t, 2)
	src := putCreature(t, h, 0, "Source", "2/2", "")
	// TargetedController resolves to seat 1; the two candidate controllers
	// deliberately differ so the filter's referent is observable.
	anchor := putCreature(t, h, 1, "Anchor", "1/3", "")
	chosenDamager := putCreature(t, h, 1, "ChosenDamager", "4/4", "")
	otherDamager := putCreature(t, h, 0, "OtherDamager", "5/5", "")
	if h.g.Obj(anchor).Controller == h.g.Obj(otherDamager).Controller ||
		h.g.Obj(anchor).Controller != h.g.Obj(chosenDamager).Controller {
		t.Fatal("precondition: the target anchor and chosen damager must share a controller distinct from the other damager")
	}
	c := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: anchor}}}
	Resolve(h, c, sa(t,
		"DB$ EachDamage | DefinedDamagers$ Valid Creature.ControlledBy TargetedController | Defined$ Self | NumDmg$ 1"))
	got := damageEvents(h)
	if len(got) != 2 || got[0].Obj != src || got[0].Amount != 1 || got[1].Obj != src || got[1].Amount != 1 {
		t.Fatalf("damage = %+v, want the two matching seat-1 creatures (anchor and damager) to hit Source for 1 each", got)
	}
	if eachDamageHasNote(h) {
		t.Fatalf("log = %+v, supported TargetedController filter must not emit a Note", h.log)
	}
}
