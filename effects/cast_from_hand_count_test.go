package effects

import (
	"testing"
)

// The Count$wasCastFromYourHandByYou branch head (the Myojin cycle's
// K:etbCounter CheckSVar$ gate: "enters with a divinity counter on it if you
// cast it from your hand", 12 corpus carriers), unit-tested at the eval level
// the Count$Foretold tests use. Grammar: <head>.<ifTrue>.<ifFalse>, both
// operands through the shared operand machinery. The provenance itself is
// Host.WasCastFromHandByYou's log scan — pinned end to end on the real engine
// in rules (the Myojin / Hotheaded / Freestrider corpus tests).

func TestWasCastFromHandHeadSelectsBranchByTheCastProvenance(t *testing.T) {
	h, c := fixtureHost(t)
	// Never cast (cheated into play): the false branch.
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.1.0"); got != 0 {
		t.Errorf("uncast wasCastFromYourHandByYou.1.0 = %d, want 0", got)
	}
	// Cast from hand by the controller: the true branch.
	h.castFromHand = true
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.1.0"); got != 1 {
		t.Errorf("cast-from-hand wasCastFromYourHandByYou.1.0 = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.0.1"); got != 0 {
		t.Errorf("cast-from-hand wasCastFromYourHandByYou.0.1 = %d, want the true branch's 0", got)
	}
	// A copy was never cast: the false branch even with a true Host read
	// (the same IsCopy guard the wasCastFromGraveyard case takes).
	h.g.Obj(c.Source).IsCopy = true
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.1.0"); got != 0 {
		t.Errorf("copy wasCastFromYourHandByYou.1.0 = %d, want 0", got)
	}
	// A source object that is gone: the false branch.
	h.castFromHand = true
	c2 := &Ctx{Source: 999, Controller: 0}
	if got := EvalCount(h, c2, "Count$wasCastFromYourHandByYou.1.0"); got != 0 {
		t.Errorf("absent-source wasCastFromYourHandByYou.1.0 = %d, want 0", got)
	}
}

func TestWasCastFromHandHeadResolvesSVariantOperands(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"X": "Count$xPaid"}
	h.castFromHand = true
	// The Myojin SVar bodies are the literal pair (1.0); an SVar-named
	// operand resolves through the face's table the way Count$Foretold.X.1
	// does: the true branch reads the announced value (3 here).
	c.X = 3
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.X.1"); got != 3 {
		t.Errorf("cast-from-hand wasCastFromYourHandByYou.X.1 = %d, want the paid X's 3", got)
	}
	h.castFromHand = false
	if got := EvalCount(h, c, "Count$wasCastFromYourHandByYou.X.1"); got != 1 {
		t.Errorf("uncast wasCastFromYourHandByYou.X.1 = %d, want the false branch's 1", got)
	}
}
