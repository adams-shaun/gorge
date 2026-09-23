package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestTriggerRememberedRefProperty pins the TriggerRemembered$<Property>
// count-reference family (task triggerremembered1): before the ref name was
// admitted, every one of the 42 corpus carriers failed closed to (0, false)
// at refTargets' switch, so a body sized by it -- Loamcrafter Faun's "up to
// that many target nonland permanent cards", Cemetery Desecrator's "X is the
// mana value of the exiled card" -- read zero.
//
// Forge's TriggerRemembered names the trigger's own RememberObjects$ capture
// (the set this engine threads through Ctx.Remembered at resolution), i.e.
// the same slot TriggeredCard$... reads. The fixture therefore asserts the
// two refs agree property-for-property, so a future edit that widens one
// without the other fails here rather than silently on a real board.
func TestTriggerRememberedRefProperty(t *testing.T) {
	h := newHost(t, 2)
	// A 3/4 for {2}: enough distinct properties that a wrong property arm
	// (power read where toughness was asked, etc.) cannot pass.
	beast := mkCard(t, "Name:Beast\nManaCost:2\nTypes:Creature Beast\nPT:3/4\nOracle:x\n")
	// A second remembered card with a different mana value and a counter, so
	// the summed properties (CardManaCost, Amount) and CardCounters.<KIND>
	// each have a non-trivial, non-uniform answer.
	talisman := mkCard(t, "Name:Talisman\nManaCost:4\nTypes:Artifact\nOracle:x\n")
	ao := h.g.AddObject(beast, 0)
	bo := h.g.AddObject(talisman, 0)
	h.g.Obj(bo.ID).AddCounter("P1P1", 2)
	// The trigger-remembered set: the two objects above. Ctx.Captured is the
	// trigger's own event capture; TriggerRemembered reads Remembered, exactly
	// like TriggeredCard, so it is populated the same way a real
	// ImmediateTrigger instance populates it (cc.Remembered = the
	// introspected set).
	rem := []state.Target{{Obj: ao.ID}, {Obj: bo.ID}}
	c := &Ctx{Source: ao.ID, Controller: 0, Remembered: rem, Captured: rem}

	// Precondition: the fixture must actually carry the values the assertions
	// depend on. A vacuous board (no objects, no counters) would otherwise let
	// the whole test pass with the feature unregistered.
	if got := EvalCount(h, c, "TriggeredCard$Amount"); got != 2 {
		t.Fatalf("precondition: TriggeredCard$Amount = %d, want 2 (remembered set not populated)", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CardCounters.P1P1"); got != 2 {
		t.Fatalf("precondition: TriggeredCard$CardCounters.P1P1 = %d, want 2 (counter not placed)", got)
	}

	for _, tc := range []struct {
		expr string
		want int32
	}{
		// Amount: the number of remembered objects (Loamcrafter Faun, Miasma
		// Demon, Ravenous Rotbelly, Caldera Breaker, ...).
		{"TriggerRemembered$Amount", 2},
		// CardPower / CardToughness sum over the remembered objects, face plus
		// marked P1P1 counters: (3 + 0+2) and (4 + 0+2).
		{"TriggerRemembered$CardPower", 5},
		{"TriggerRemembered$CardToughness", 6},
		// CardManaCost (and its LKI spelling, the same read) sum the face's
		// mana values: 2 + 4.
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
		// not model; both stay zero, but the ref itself is now admitted.
		{"TriggerRemembered$CastTotalManaSpent", 0},
		// Every property TriggeredCard$ exposes on the same slot must agree.
		{"TriggeredCard$Amount", 2},
		{"TriggeredCard$CardPower", 5},
		{"TriggeredCard$CardToughness", 6},
		{"TriggeredCard$CardManaCost", 6},
		{"TriggeredCard$CardCounters.P1P1", 2},
		// An unknown ref stays zero.
		{"UnknownRef$CardPower", 0},
	} {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}

	// The regression that the ticket is about: with the head unread, the
	// DynamicAmount form a TargetMax$ X rider consumes answers zero. Assert
	// the whole expression the corpus writes, not just the raw head, so the
	// path EvalCountOK -> evalRefProperty is exercised.
	if got, ok := EvalCountOK(h, c, "TriggerRemembered$Amount"); !ok || got != 2 {
		t.Errorf("EvalCountOK(TriggerRemembered$Amount) = (%d, %v), want (2, true): head not admitted", got, ok)
	}
}
