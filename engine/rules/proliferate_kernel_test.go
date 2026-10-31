package rules

// The proliferate ask
// offers exactly the permanents and players carrying a live counter (CR
// 701.27a; a slot drained to zero is not a counter), the answered objects
// AND players each gain one of every kind they already have (Amount$ N:
// N of each), and RememberPut$ records the recipients for the chain.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const creatureKernelSrc = "ManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n"

func TestProliferateAsksAndAppliesObjectsAndPlayers(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	carrier := putCardIn(t, e, 0, cardOfSrc(t, "Name:Carrier\n"+creatureKernelSrc), state.ZBattlefield, false)
	bare := putCardIn(t, e, 0, cardOfSrc(t, "Name:Bare\n"+creatureKernelSrc), state.ZBattlefield, false)
	other := putCardIn(t, e, 1, cardOfSrc(t, "Name:Other\n"+creatureKernelSrc), state.ZBattlefield, false)
	e.G.Obj(carrier).AddCounter("P1P1", 2)
	e.G.Obj(carrier).AddCounter("CHARGE", 1)
	e.G.Obj(other).AddCounter("P1P1", 1)
	e.G.Players[1].AddCounter("POISON", 1)
	spell := putCardIn(t, e, 0, sorceryCard(t, "Spread", "A:SP$ Proliferate"), state.ZHand, false)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "proliferate")
	if d.Kind != decision.KChoose || d.Min != 0 || d.Max != 3 || d.Player != 0 || len(d.Options) != 3 {
		t.Fatalf("ask = %+v, want a Min 0 / Max 3 KChoose for seat 0 over 3 eligible recipients", d)
	}
	if d.Options[0].Obj != carrier || d.Options[1].Obj != other || d.Options[2].Obj != 0 || d.Options[2].Player != 1 {
		t.Fatalf("options = %+v, want carrier, other, then player 1", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == bare {
			t.Fatal("a recipient carrying no counter was offered")
		}
	}
	if d = answerDecision(t, e, d, 0, 1, 2); d != nil {
		t.Fatalf("unexpected ask after proliferate: %+v", d)
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3", got)
	}
	if got := e.G.Obj(carrier).Counter("CHARGE"); got != 2 {
		t.Fatalf("carrier CHARGE = %d, want 2", got)
	}
	if got := e.G.Obj(other).Counter("P1P1"); got != 2 {
		t.Fatalf("other P1P1 = %d, want 2", got)
	}
	if got := e.G.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("player POISON = %d, want 2", got)
	}
	saw := false
	for _, ev := range eventsOfKindFrom(e, from, events.PlayerCounterChange) {
		if ev.Player == 1 && ev.Counter == "POISON" && ev.Amount == 1 {
			saw = true
		}
	}
	if !saw {
		t.Fatal("no PlayerCounterChange for the chosen player")
	}
}

func TestProliferateAmountAndRememberPut(t *testing.T) {
	t.Parallel()
	t.Run("Amount$ 2", func(t *testing.T) {
		t.Parallel()
		e := kernelEngine(t, 2)
		carrier := putCardIn(t, e, 0, cardOfSrc(t, "Name:Carrier\n"+creatureKernelSrc), state.ZBattlefield, false)
		e.G.Obj(carrier).AddCounter("P1P1", 1)
		spell := putCardIn(t, e, 0, sorceryCard(t, "Spread", "A:SP$ Proliferate | Amount$ 2"), state.ZHand, false)
		d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "proliferate")
		// CR 701.34a: two proliferate actions, two asks.
		d = requireResumeAsk(t, answerDecision(t, e, d, objOptionIndex(t, d, carrier)), "proliferate")
		if d = answerDecision(t, e, d, objOptionIndex(t, d, carrier)); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
			t.Fatalf("carrier P1P1 = %d, want 3 (+1 twice)", got)
		}
	})
	t.Run("RememberPut$", func(t *testing.T) {
		t.Parallel()
		e := kernelEngine(t, 2)
		carrier := putCardIn(t, e, 0, cardOfSrc(t, "Name:Carrier\n"+creatureKernelSrc), state.ZBattlefield, false)
		other := putCardIn(t, e, 0, cardOfSrc(t, "Name:Other\n"+creatureKernelSrc), state.ZBattlefield, false)
		e.G.Obj(carrier).AddCounter("P1P1", 1)
		e.G.Obj(other).AddCounter("P1P1", 1)
		spell := putCardIn(t, e, 0, sorceryCard(t, "Spread",
			"A:SP$ Proliferate | RememberPut$ True | SubAbility$ DBPump",
			"SVar:DBPump:DB$ PutCounter | Defined$ Remembered | CounterType$ CHARGE | CounterNum$ 1"), state.ZHand, false)
		d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "proliferate")
		if d = answerDecision(t, e, d, objOptionIndex(t, d, carrier)); d != nil {
			t.Fatalf("unexpected ask: %+v", d)
		}
		if got := e.G.Obj(carrier).Counter("CHARGE"); got != 1 {
			t.Fatalf("carrier CHARGE = %d, want 1 (it was remembered as a recipient)", got)
		}
		if got := e.G.Obj(other).Counter("CHARGE"); got != 0 {
			t.Fatalf("unchosen permanent CHARGE = %d, want 0 (only recipients are remembered)", got)
		}
	})
}

func TestProliferateDrainedCounterSlotIsNotEligible(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	drained := putCardIn(t, e, 0, cardOfSrc(t, "Name:Drained\n"+creatureKernelSrc), state.ZBattlefield, false)
	mixed := putCardIn(t, e, 0, cardOfSrc(t, "Name:Mixed\n"+creatureKernelSrc), state.ZBattlefield, false)
	e.G.Obj(drained).AddCounter("P1P1", 1)
	e.G.Obj(drained).AddCounter("P1P1", -1)
	e.G.Obj(mixed).AddCounter("P1P1", 1)
	e.G.Obj(mixed).AddCounter("CHARGE", 1)
	e.G.Obj(mixed).AddCounter("CHARGE", -1)
	e.G.Players[1].AddCounter("POISON", 1)
	e.G.Players[1].AddCounter("POISON", -1)
	if len(e.G.Obj(drained).Counters) == 0 || len(e.G.Players[1].Counters) == 0 {
		t.Fatal("precondition: the drained slots were pruned (state changed)")
	}
	spell := putCardIn(t, e, 0, sorceryCard(t, "Spread", "A:SP$ Proliferate"), state.ZHand, false)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "proliferate")
	if len(d.Options) != 1 || d.Options[0].Obj != mixed {
		t.Fatalf("options = %+v, want only the mixed permanent (no drained object, no drained player)", d.Options)
	}
	if d = answerDecision(t, e, d, 0); d != nil {
		t.Fatalf("unexpected ask: %+v", d)
	}
	if got := e.G.Obj(mixed).Counter("P1P1"); got != 2 {
		t.Fatalf("mixed P1P1 = %d, want 2", got)
	}
	if got := e.G.Obj(mixed).Counter("CHARGE"); got != 0 {
		t.Fatalf("mixed CHARGE = %d, want 0 (a drained kind must not receive +1)", got)
	}
	for _, ev := range eventsOfKindFrom(e, from, events.CounterChange) {
		if ev.Counter == "CHARGE" {
			t.Fatalf("proliferate emitted a CounterChange for a drained kind: %+v", ev)
		}
	}
	if pc := eventsOfKindFrom(e, from, events.PlayerCounterChange); len(pc) != 0 {
		t.Fatalf("proliferate emitted a PlayerCounterChange for a drained player slot: %+v", pc)
	}
}
