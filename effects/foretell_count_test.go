package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The Count$Foretold branch head (CR 702.126) and the Count$NotedNumber
// reader, unit-tested at the eval level the Count$Compare tests use. The
// grammar is `Foretold.<ifTrue>.<ifFalse>`: each operand is a literal or an
// SVar name resolved through evalCountOperand (Starnheim Unleashed's
// Count$Foretold.X.1 reads the announced X through the face's SVar table); a
// body missing a branch is a corpus bug and fails closed.

func TestForetoldHeadSelectsBranchByTheCastFlag(t *testing.T) {
	h, c := fixtureHost(t)
	// Un-flagged source: the false branch.
	if got := EvalCount(h, c, "Count$Foretold.1.0"); got != 0 {
		t.Errorf("un-flagged Foretold.1.0 = %d, want 0", got)
	}
	// Flagged source: the true branch.
	src := h.g.Obj(c.Source)
	src.CastFlags = state.FlagForetold
	if got := EvalCount(h, c, "Count$Foretold.1.0"); got != 1 {
		t.Errorf("flagged Foretold.1.0 = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$Foretold.0.1"); got != 0 {
		t.Errorf("flagged Foretold.0.1 = %d, want the true branch's 0", got)
	}
	// A source object that is gone (never cast): the false branch.
	c2 := &Ctx{Source: 999, Controller: 0}
	if got := EvalCount(h, c2, "Count$Foretold.1.0"); got != 0 {
		t.Errorf("absent-source Foretold.1.0 = %d, want 0", got)
	}
}

func TestForetoldHeadResolvesSVariantOperands(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"X": "Count$xPaid"}
	h.g.Obj(c.Source).CastFlags = state.FlagForetold
	// Starnheim Unleashed's own shape: SVar:Y:Count$Foretold.X.1 -- the true
	// branch resolves the SVar-named X (the paid {X}, 3 here), the false
	// branch the literal 1.
	c.X = 3
	if got := EvalCount(h, c, "Count$Foretold.X.1"); got != 3 {
		t.Errorf("flagged Foretold.X.1 = %d, want the paid X's 3", got)
	}
	if got := EvalCount(h, c, "Count$Foretold.1.0"); got != 1 {
		t.Errorf("flagged Foretold.1.0 = %d, want 1", got)
	}
	// The SVar operand recurses through the announced value with the shared
	// depth discipline (maxCountDepth): a self-referential table TERMINATES
	// deterministically -- the capped recursion degrades the true branch's
	// operand to 0, never an overflow -- and the false branch still resolves.
	c.SVars["Loop"] = "Count$Foretold.Loop.7"
	if got := EvalCount(h, c, "Count$Foretold.Loop.7"); got != 0 {
		t.Errorf("self-referential Foretold operand = %d, want the depth-capped 0", got)
	}
}

func TestForetoldHeadFailsClosedOnAMissingBranch(t *testing.T) {
	h, c := fixtureHost(t)
	for _, body := range []string{"Count$Foretold.1", "Count$Foretold.1.", "Count$Foretold"} {
		if _, ok := EvalCountOK(h, c, body); ok {
			t.Errorf("%s resolved, want the fail-closed (0,false)", body)
		}
	}
}

func TestNotedNumberHeadReadsTheCardNote(t *testing.T) {
	h, c := fixtureHost(t)
	if got := EvalCount(h, c, "Count$NotedNumber"); got != 0 {
		t.Errorf("un-noted NotedNumber = %d, want 0", got)
	}
	h.g.Obj(c.Source).NotedNumber = 4
	if got := EvalCount(h, c, "Count$NotedNumber"); got != 4 {
		t.Errorf("noted NotedNumber = %d, want 4", got)
	}
}

func TestBareConditionForetoldGatesOnTheCastFlag(t *testing.T) {
	h, c := fixtureHost(t)
	s := sa(t, "SP$ Draw | Condition$ Foretold | NumCards$ 1")
	if met, ok := conditionMet(h, c, s); met || !ok {
		t.Errorf("un-foretold Condition$ Foretold = (%v,%v), want (false,true)", met, ok)
	}
	h.g.Obj(c.Source).CastFlags = state.FlagForetold
	if met, ok := conditionMet(h, c, s); !met || !ok {
		t.Errorf("foretold Condition$ Foretold = (%v,%v), want (true,true)", met, ok)
	}
}
