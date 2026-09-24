package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestTriggerRememberedRefProperty pins the TriggerRemembered$<Property>
// count-reference family (task triggerremembered1) and, specifically, its
// capture-excluded referent mapping (task agent-20260918T195920Z-2fd3b568).
//
// Forge's TriggerRemembered names the trigger's own RememberObjects$ capture
// -- Forge's host remembered list, which never contains the event object the
// trigger fired on. rules seeds a firing trigger's ctx with
// Remembered == Captured == the trigger's event capture, and the chain's
// remember rider (Loamcrafter Faun's RememberDiscarded$) appends its objects
// to Remembered, so a plain Ctx.Remembered read overcounts by that one
// capture: Loamcrafter would offer discards+1 picks and let the player return
// one more than they paid for. The fixture therefore seeds the ctx the way a
// real ETB trigger does -- Captured = the ETB'd source, Remembered = the
// source plus the two chain objects -- and asserts TriggerRemembered counts
// exactly the two chain objects.
//
// Before the ref name was admitted at all, every one of the 42 corpus
// carriers failed closed to (0, false) at refTargets' switch, so a body sized
// by it -- Loamcrafter Faun's "up to that many target nonland permanent
// cards", Cemetery Desecrator's "X is the mana value of the exiled card" --
// read zero.
func TestTriggerRememberedRefProperty(t *testing.T) {
	h := newHost(t, 2)
	// A 3/4 for {2}: enough distinct properties that a wrong property arm
	// (power read where toughness was asked, etc.) cannot pass.
	beast := mkCard(t, "Name:Beast\nManaCost:2\nTypes:Creature Beast\nPT:3/4\nOracle:x\n")
	// A second remembered card with a different mana value and a counter, so
	// the summed properties (CardManaCost, Amount) and CardCounters.<KIND>
	// each have a non-trivial, non-uniform answer.
	talisman := mkCard(t, "Name:Talisman\nManaCost:4\nTypes:Artifact\nOracle:x\n")
	// The fire-time capture: the object the trigger fired on (Loamcrafter
	// Faun itself, the ETB'd source). It is NOT part of the remembered set
	// the chain introspects.
	source := mkCard(t, "Name:Source\nTypes:Creature Faun\nPT:3/3\nOracle:x\n")
	ao := h.g.AddObject(beast, 0)
	bo := h.g.AddObject(talisman, 0)
	so := h.g.AddObject(source, 0)
	h.g.Obj(bo.ID).AddCounter("P1P1", 2)
	// The real chain ctx: Captured = the ETB'd source; Remembered = the
	// capture PLUS the objects the remember rider appended (the two chain
	// objects), capture first, the order rules' triggerRememberedFor seeds
	// and the rider appends to.
	capture := state.Target{Obj: so.ID}
	rem := []state.Target{capture, {Obj: ao.ID}, {Obj: bo.ID}}
	c := &Ctx{Source: so.ID, Controller: 0, Remembered: rem, Captured: []state.Target{capture}}

	// Precondition: the fixture must actually carry the values the assertions
	// depend on. A vacuous board (no objects, no counters) or a ctx whose
	// capture is disjoint from Remembered would let the test pass against the
	// wrong mapping; assert both halves of the shape here.
	if got := EvalCount(h, c, "TriggeredCard$Amount"); got != 3 {
		t.Fatalf("precondition: TriggeredCard$Amount = %d, want 3 (Remembered list not seeded with capture+chain)", got)
	}
	if got := EvalCount(h, c, "TriggerRemembered$Amount"); got != 2 {
		t.Fatalf("precondition: TriggerRemembered$Amount = %d, want 2 (capture not excluded)", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CardCounters.P1P1"); got != 2 {
		t.Fatalf("precondition: TriggeredCard$CardCounters.P1P1 = %d, want 2 (counter not placed)", got)
	}

	for _, tc := range []struct {
		expr string
		want int32
	}{
		// Amount: the number of remembered objects EXCLUDING the fire-time
		// capture (Loamcrafter Faun, Miasma Demon, Ravenous Rotbelly,
		// Caldera Breaker, ...). Two chain objects, not three.
		{"TriggerRemembered$Amount", 2},
		// CardPower / CardToughness sum over the capture-excluded objects,
		// face plus marked P1P1 counters: (3 + 0+2) and (4 + 0+2). The
		// capture (3/3) is excluded, so 5/6 not 8/9.
		{"TriggerRemembered$CardPower", 5},
		{"TriggerRemembered$CardToughness", 6},
		// CardManaCost (and its LKI spelling, the same read) sum the face's
		// mana values: 2 + 4 (the capture has no printed cost).
		{"TriggerRemembered$CardManaCost", 6},
		{"TriggerRemembered$CardManaCostLKI", 6},
		// CardCounters.<KIND> sums one counter kind: 0 + 2.
		{"TriggerRemembered$CardCounters.P1P1", 2},
		{"TriggerRemembered$CardCounters.P1P1/HalfDown", 1},
		// The /Op suffix applies through the same arithmetic.
		{"TriggerRemembered$Amount/Twice", 4},
		// CastTotalManaSpent is an Object field read, zero for these
		// never-cast objects -- it must resolve (not fail closed) and answer
		// zero. GreatestCardManaCost is an extreme aggregate this build does
		// not model; both stay zero, but the ref itself is admitted.
		{"TriggerRemembered$CastTotalManaSpent", 0},
		// The adjacent refs on the same ctx keep their own binding: the
		// plain TriggeredCard read is the WHOLE Remembered list (capture
		// included), proving the exclusion is TriggerRemembered's alone.
		{"TriggeredCard$Amount", 3},
		{"TriggeredCard$CardPower", 8},
		{"TriggeredCard$CardToughness", 9},
		{"TriggeredCard$CardManaCost", 6},
		{"TriggeredCard$CardCounters.P1P1", 2},
		{"RememberedLKI$Amount", 3},
		// An unknown ref stays zero.
		{"UnknownRef$CardPower", 0},
	} {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}

	// The instance-ctx shape effImmediateTrigger builds: Remembered is
	// already the capture-excluded parent and Captured is the OUTER trigger's
	// capture, disjoint from it. The exclusion must be idempotent here --
	// applying it again must not drop a chain object.
	inst := &Ctx{Source: so.ID, Controller: 0,
		Remembered: []state.Target{{Obj: ao.ID}, {Obj: bo.ID}},
		Captured:   []state.Target{capture}}
	if got := EvalCount(h, inst, "TriggerRemembered$Amount"); got != 2 {
		t.Errorf("instance ctx TriggerRemembered$Amount = %d, want 2 (exclusion not idempotent)", got)
	}

	// The regression that the ticket is about: with the head unread, the
	// DynamicAmount form a TargetMax$ X rider consumes answers zero. Assert
	// the whole expression the corpus writes, not just the raw head, so the
	// path EvalCountOK -> evalRefProperty is exercised.
	if got, ok := EvalCountOK(h, c, "TriggerRemembered$Amount"); !ok || got != 2 {
		t.Errorf("EvalCountOK(TriggerRemembered$Amount) = (%d, %v), want (2, true): head not admitted", got, ok)
	}
	// Pin the EXOTIC verdicts the brief asks about, because a plain
	// EvalCount assertion cannot tell a modelled zero from a fail-closed
	// one. CastTotalManaSpent and CardManaCostLKI are both modelled by the
	// shared property switch today (the brief listed them as still-degraded;
	// re-measured here they resolve), while GreatestCardManaCost and
	// CardTypes are genuinely absent from evalRefProperty and stay
	// fail-closed (ok=false).
	for _, tc := range []struct {
		expr string
		want int32
		ok   bool
	}{
		{"TriggerRemembered$CastTotalManaSpent", 0, true},
		{"TriggerRemembered$CardManaCostLKI", 6, true},
		{"TriggerRemembered$GreatestCardManaCost", 0, false},
		{"TriggerRemembered$CardTypes", 0, false},
	} {
		got, ok := EvalCountOK(h, c, tc.expr)
		if got != tc.want || ok != tc.ok {
			t.Errorf("EvalCountOK(%s) = (%d, %v), want (%d, %v)", tc.expr, got, ok, tc.want, tc.ok)
		}
	}
}
