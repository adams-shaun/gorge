package rules

// CR 601.2h: "The player pays the total cost" -- the objects a cost
// sacrifices, discards or exiles are chosen as it is paid: after the targets
// (601.2c) and after the mana abilities of 601.2g. One object can pay only
// one cost. Real corpus cards read as script text (Fling); the Eldrazi Spawn
// is the potential-plan tests' ppSpawn (the corpus token's exact ability).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR601_2hTargetsAreChosenBeforeTheSacrifice: Fling with {R}{R} floating
// and Grizzly Bears. After the push, the first choice is Fling's target; the
// sacrifice is chosen afterwards, at payment.
func TestCR601_2hTargetsAreChosenBeforeTheSacrifice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "f/fling.txt")))
	fling := e.G.Zone(state.ZHand, 0)[0]
	bear := battlePerm(t, e, 0, corpusCardText(t, "g/grizzly_bears.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.askPriority(0)
	cr601Choose(t, e, "cast Fling", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == fling })
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("CR 601.2c before 601.2h: Fling's first choice is %+v, want its target", d)
	}
	cr601Choose(t, e, "target", func(o decision.Option) bool { return o.Kind == "player" && o.Label == "b" })
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != bear {
		t.Fatalf("after the target, want the sacrifice of Grizzly Bears, got %+v", d)
	}
	cr601Choose(t, e, "sacrifice", func(o decision.Option) bool { return o.Obj == bear })
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Errorf("Grizzly Bears is in %s, want the graveyard", z)
	}
	if z := e.G.Obj(fling).Zone; z != state.ZStack {
		t.Errorf("Fling is in %s, want the stack", z)
	}
}

// TestCR601_2hOneObjectPaysOneCost: Fling over an Eldrazi Spawn ("Sacrifice
// this creature: Add {C}"), a creature and one red source, cast through its
// payment plan (Red Rock for {R}, the Spawn for {1}). The plan's mana
// abilities run first (601.2g), so the Spawn is gone when the sacrifice cost
// is chosen: it is not offered, the Bear pays the sacrifice, and the Spawn is
// sacrificed exactly once. (Before the fix the sacrifice was chosen first,
// the Spawn was offered, and choosing it sacrificed it twice.)
func TestCR601_2hOneObjectPaysOneCost(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "f/fling.txt")))
	fling := e.G.Zone(state.ZHand, 0)[0]
	spawn := battlePerm(t, e, 0, ppSpawn)
	bear := battlePerm(t, e, 0, corpusCardText(t, "g/grizzly_bears.txt"))
	battlePerm(t, e, 0, "Name:Red Rock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	e.askPriority(0)
	d := e.Pending()
	e.EnsurePaymentActions()
	var action *decision.PaymentAction
	for i := range d.PaymentActions {
		if a := &d.PaymentActions[i]; a.Cast.Object == fling && len(a.Plans) > 0 {
			action = a
		}
	}
	if action == nil {
		t.Fatalf("no planned payment for Fling: %+v", d.PaymentActions)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Payment: &decision.PaymentSelection{ActionID: action.ID, Plan: action.Plans[0]}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		if d.Kind == decision.KTarget {
			cr601Choose(t, e, "target", func(o decision.Option) bool { return o.Kind == "player" && o.Label == "b" })
			continue
		}
		// Choose the Spawn whenever it is offered: that is the double use
		// CR 601.2h forbids (its mana ability pays {1} of the plan).
		pick := bear
		for _, o := range d.Options {
			if o.Kind == "sacrifice" && o.Obj == spawn {
				t.Errorf("CR 601.2h: the sacrifice cost offers the Spawn the payment plan sacrifices for mana (option %d)", o.Index)
				pick = spawn
			}
		}
		cr601Choose(t, e, "sacrifice", func(o decision.Option) bool { return o.Kind == "sacrifice" && o.Obj == pick })
	}
	sacrificed := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == spawn && ev.From == state.ZBattlefield {
			sacrificed++
		}
	}
	if sacrificed != 1 {
		t.Errorf("CR 601.2h: the Spawn left the battlefield %d times, want once (one object pays one cost)", sacrificed)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Errorf("Grizzly Bears is in %s, want the graveyard (it paid the sacrifice)", z)
	}
	if z := e.G.Obj(fling).Zone; z != state.ZStack {
		t.Errorf("Fling is in %s, want the stack", z)
	}
}
