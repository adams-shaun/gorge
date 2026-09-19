// The BARE wasCastFromYourHand predicate family (task castprov3), unit-tested
// at the effects-side eval level the ByYou twin (castprov1) uses: the Count$
// branch head's dotted grammar (see_the_truth's
// SVar:X:Count$wasCastFromYourHand.1.3), the ConditionPresent$ gate helper
// (otterball_antics' `Card.wasCast+!wasCastFromYourHand`), and the
// UnknownPredicates guard reading the token-STRIPPED spec. The provenance
// itself is Host.WasCastFromHand's log scan — pinned end to end on the real
// engine in rules (the Vega / Otterball / See the Truth corpus tests).

package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestWasCastFromHandHeadBareSelectsBranchByTheCastProvenance(t *testing.T) {
	h, c := fixtureHost(t)
	// Never cast (cheated into play): the false branch.
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.1.0"); got != 0 {
		t.Errorf("uncast wasCastFromYourHand.1.0 = %d, want 0", got)
	}
	// Cast from a hand (any caster — the bare token carries no player
	// scoping): the true branch.
	h.castFromHand = true
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.1.0"); got != 1 {
		t.Errorf("cast-from-hand wasCastFromYourHand.1.0 = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.0.1"); got != 0 {
		t.Errorf("cast-from-hand wasCastFromYourHand.0.1 = %d, want the true branch's 0", got)
	}
	// A copy was never cast: the false branch even with a true Host read
	// (the same IsCopy guard the ByYou head takes).
	h.g.Obj(c.Source).IsCopy = true
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.1.0"); got != 0 {
		t.Errorf("copy wasCastFromYourHand.1.0 = %d, want 0", got)
	}
	// A source object that is gone: the false branch.
	h.castFromHand = true
	c2 := &Ctx{Source: 999, Controller: 0}
	if got := EvalCount(h, c2, "Count$wasCastFromYourHand.1.0"); got != 0 {
		t.Errorf("absent-source wasCastFromYourHand.1.0 = %d, want 0", got)
	}
}

func TestWasCastFromHandHeadBareResolvesSVariantOperands(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"X": "Count$xPaid"}
	h.castFromHand = true
	// see_the_truth's body is the literal pair (1.3); an SVar-named operand
	// resolves through the face's table the way Count$Foretold.X.1 does: the
	// true branch reads the announced value (3 here).
	c.X = 3
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.X.1"); got != 3 {
		t.Errorf("cast-from-hand wasCastFromYourHand.X.1 = %d, want the paid X's 3", got)
	}
	h.castFromHand = false
	if got := EvalCount(h, c, "Count$wasCastFromYourHand.X.1"); got != 1 {
		t.Errorf("uncast wasCastFromYourHand.X.1 = %d, want the false branch's 1", got)
	}
}

// TestBareConditionPresentGateResolvesTheHandProvenance pins otterball
// antics' gate shape through the fake host: `ConditionDefined$ Self |
// ConditionPresent$ Card.wasCast+!wasCastFromYourHand | ConditionCompare$
// EQ1` is met exactly when the resolving spell on the stack was cast from
// anywhere OTHER than a hand (the fake's flag false), and its positive
// mirror met exactly when it was cast from a hand.
func TestBareConditionPresentGateResolvesTheHandProvenance(t *testing.T) {
	h, c := fixtureHost(t)
	// wasCast needs the source to BE a spell on the stack.
	h.g.Obj(c.Source).Zone = state.ZStack
	negated := sa(t, "DB$ PutCounter | Defined$ Remembered | ConditionDefined$ Self | "+
		"ConditionPresent$ Card.wasCast+!wasCastFromYourHand | ConditionCompare$ EQ1")
	h.castFromHand = false
	if met, resolved := conditionMet(h, c, negated); !met || !resolved {
		t.Fatalf("non-hand cast vs the negated gate: met=%v resolved=%v, want true true", met, resolved)
	}
	h.castFromHand = true
	if met, resolved := conditionMet(h, c, negated); met || !resolved {
		t.Fatalf("hand cast vs the negated gate: met=%v resolved=%v, want false true", met, resolved)
	}
	positive := sa(t, "DB$ PutCounter | ConditionDefined$ Self | "+
		"ConditionPresent$ Card.wasCast+wasCastFromYourHand | ConditionCompare$ EQ1")
	h.castFromHand = true
	if met, resolved := conditionMet(h, c, positive); !met || !resolved {
		t.Fatalf("hand cast vs the positive gate: met=%v resolved=%v, want true true", met, resolved)
	}
	h.castFromHand = false
	if met, resolved := conditionMet(h, c, positive); met || !resolved {
		t.Fatalf("non-hand cast vs the positive gate: met=%v resolved=%v, want false true", met, resolved)
	}
	// An uncast (never on the stack) Self fails wasCast itself in both
	// polarities: resolved, unmet.
	h.g.Obj(c.Source).Zone = state.ZBattlefield
	h.castFromHand = true
	if met, resolved := conditionMet(h, c, positive); met || !resolved {
		t.Fatalf("battlefield Self vs the positive gate: met=%v resolved=%v, want false true (wasCast denies)", met, resolved)
	}
}

// TestBareConditionPresentGateStripsTheTokenForUnknownPredicates pins the
// guard contract: a spec carrying the bare token but an otherwise-known
// remainder RESOLVES (the token is stripped before the UnknownPredicates
// read), while an unknown REMAINDER behind the token stays unresolved (the
// gate must not silently stop subs an unreadable spec used to stop).
func TestBareConditionPresentGateStripsTheTokenForUnknownPredicates(t *testing.T) {
	h, c := fixtureHost(t)
	h.g.Obj(c.Source).Zone = state.ZStack
	h.castFromHand = false
	resolvable := sa(t, "DB$ PutCounter | ConditionDefined$ Self | "+
		"ConditionPresent$ Card.!wasCastFromYourHand | ConditionCompare$ EQ1")
	if met, resolved := conditionMet(h, c, resolvable); !met || !resolved {
		t.Fatalf("bare token over a known remainder: met=%v resolved=%v, want true true", met, resolved)
	}
	unreadable := sa(t, "DB$ PutCounter | ConditionDefined$ Self | "+
		"ConditionPresent$ Card.!wasCastFromYourHand+nosuchpredicate | ConditionCompare$ EQ1")
	if met, resolved := conditionMet(h, c, unreadable); met || resolved {
		t.Fatalf("unknown remainder behind the bare token: met=%v resolved=%v, want false false", met, resolved)
	}
}
