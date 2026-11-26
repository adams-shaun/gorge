package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// UnlessPayer$ EnchantedController names the controller of the permanent the
// source is attached to NOW (the source's live AttachedTo, which leaving the
// battlefield clears, CR 704.5m). An Aura that has left the battlefield with
// its trigger still on the stack names nobody, even though its last bearer is
// still around: no payer is asked and the unless body runs unpaid. Paralyze,
// Slow Motion and Parasitic Implant's corpus lines read the payer the same way
// (`Defined$ EnchantedController` / `UnlessPayer$ EnchantedController` on the
// trigger's host), so one rule serves them all, and it is the rule the dirac
// kernel's kop.RefEnchantedCtrl (dirac/trig_attachedctrl.go) reads; a rule
// that read the last bearer here (CR 608.2h) split the two engines on
// TestDefinedRefs2Lockstep.
func TestUnlessPayerEnchantedControllerLeftAuraNamesNobody(t *testing.T) {
	h := &askHost{fakeHost: fakeHost{g: state.NewGame(names(2))}}
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	aura := h.g.AddObject(mkCard(t, "Name:Slow\nManaCost:2 U\nTypes:Enchantment Aura\nOracle:x\n"), 0)
	for _, o := range []*state.Object{bear, aura} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	h.Emit(events.Event{Kind: events.Attach, Obj: aura.ID, IDs: []state.ObjID{bear.ID}})
	// a body that leaves the bearer alone: only the payer is under test
	body := sa(t, "DB$ GainLife | Defined$ You | LifeAmount$ 1 | UnlessPayer$ EnchantedController | UnlessCost$ 2")

	// Attached: the bearer's controller (seat 1), not the Aura's (seat 0).
	Resolve(h, &Ctx{Source: aura.ID, Controller: 0}, body)
	if h.asked == nil || h.asked.Player != 1 {
		t.Fatalf("attached payer = %+v, want the bearer's controller, seat 1", h.asked)
	}

	// The Aura leaves while its bearer stays: the precondition is that its
	// last bearer is still there to be read (a last-bearer rule would ask it).
	h.asked = nil
	h.Emit(events.Event{Kind: events.MoveZone, Obj: aura.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	left := h.g.Obj(aura.ID)
	if left.Zone != state.ZGraveyard || left.AttachedTo != 0 {
		t.Fatalf("precondition: Aura zone %v attachedTo %v, want the graveyard and unattached", left.Zone, left.AttachedTo)
	}
	if left.LastBearer != bear.ID || h.g.Obj(bear.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: last bearer %v (bear %v) on %v, want the bear still on the battlefield", left.LastBearer, bear.ID, h.g.Obj(bear.ID).Zone)
	}
	Resolve(h, &Ctx{Source: aura.ID, Controller: 0}, body)
	if h.asked != nil {
		t.Fatalf("a left Aura asked seat %d to pay, want nobody", h.asked.Player)
	}
}
