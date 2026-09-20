package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The Count$IfCastInOwnMainPhase / Count$InOwnMainPhase branch heads (task
// ifcastmain1): Forge's AbilityUtils reads the LIVE phase handler
// (game.getPhaseHandler(): cPhase.getPhase().isMain() && cPhase.isPlayerTurn),
// not a stamp of the cast's phase, plus -- for the IfCast spelling only -- the
// source must have been cast (c.wasCast()). Both operands resolve through the
// shared operand machinery; the provenance read itself is Host.WasCast, pinned
// end to end on the real engine in rules.

// TestIfCastInOwnMainPhaseHeadSelectsBranch pins all three conjuncts of the
// IfCast spelling: live main phase, live active player == controller, and the
// WasCast answer. Each leg is flipped independently so a stuck branch is
// caught.
func TestIfCastInOwnMainPhaseHeadSelectsBranch(t *testing.T) {
	h, c := fixtureHost(t)
	// Baseline: Step is not a main phase, so the not-main branch (0).
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 1 {
		t.Errorf("non-main IfCastInOwnMainPhase.2.1 = %d, want the not-main 1", got)
	}
	// Live main phase, active player is the controller, source was cast:
	// the true branch (2).
	h.g.Step = state.StepMain1
	h.g.Active = 0
	h.wasCast = true
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 2 {
		t.Errorf("own-main cast IfCastInOwnMainPhase.2.1 = %d, want 2", got)
	}
	// Main phase but NOT cast (cheated into play): the not-main branch.
	h.wasCast = false
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 1 {
		t.Errorf("own-main uncast IfCastInOwnMainPhase.2.1 = %d, want the not-main 1", got)
	}
	// Main phase but the OPPONENT is active: the not-main branch, even with
	// the source cast (Forge's isPlayerTurn conjunct).
	h.wasCast = true
	h.g.Active = 1
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 1 {
		t.Errorf("opponent-main IfCastInOwnMainPhase.2.1 = %d, want the not-main 1", got)
	}
	// Second main phase is a main phase too (Forge's isMain covers both).
	h.g.Active = 0
	h.g.Step = state.StepMain2
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 2 {
		t.Errorf("own Main2 IfCastInOwnMainPhase.2.1 = %d, want 2", got)
	}
}

// TestIfCastInOwnMainPhaseHeadCopyTakesNotMainBranch pins the copy guard: a
// stack copy was never cast, so even in the controller's live main phase with
// a WasCast read of true it takes the not-main branch (the same IsCopy guard
// the sibling provenance cases take).
func TestIfCastInOwnMainPhaseHeadCopyTakesNotMainBranch(t *testing.T) {
	h, c := fixtureHost(t)
	h.g.Step = state.StepMain1
	h.g.Active = 0
	h.wasCast = true
	h.g.Obj(c.Source).IsCopy = true
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.2.1"); got != 1 {
		t.Errorf("copy IfCastInOwnMainPhase.2.1 = %d, want the not-main 1", got)
	}
}

// TestInOwnMainPhaseHeadIgnoresWasCast pins the bare spelling: Dose of
// Dawnglow's Count$InOwnMainPhase.0.1 selects 0 in the controller's main phase
// and 1 otherwise, regardless of the WasCast answer (Forge's
// !startsWith("IfCast") disjunct skips the c.wasCast() conjunct).
func TestInOwnMainPhaseHeadIgnoresWasCast(t *testing.T) {
	h, c := fixtureHost(t)
	h.g.Step = state.StepMain1
	h.g.Active = 0
	// Even with WasCast FALSE the bare spelling takes the main-phase branch.
	h.wasCast = false
	if got := EvalCount(h, c, "Count$InOwnMainPhase.0.1"); got != 0 {
		t.Errorf("own-main uncast InOwnMainPhase.0.1 = %d, want 0", got)
	}
	h.wasCast = true
	if got := EvalCount(h, c, "Count$InOwnMainPhase.0.1"); got != 0 {
		t.Errorf("own-main cast InOwnMainPhase.0.1 = %d, want 0", got)
	}
	// A non-main step flips to the not-main branch (1).
	h.g.Step = state.StepEnd
	if got := EvalCount(h, c, "Count$InOwnMainPhase.0.1"); got != 1 {
		t.Errorf("non-main InOwnMainPhase.0.1 = %d, want 1", got)
	}
	// The opponent's main phase is not YOUR main phase.
	h.g.Step = state.StepMain1
	h.g.Active = 1
	if got := EvalCount(h, c, "Count$InOwnMainPhase.0.1"); got != 1 {
		t.Errorf("opponent-main InOwnMainPhase.0.1 = %d, want 1", got)
	}
}

// TestIfCastInOwnMainPhaseHeadResolvesSVariantOperands pins the operand
// machinery: the corpus's branch values are literals, but the shared resolver
// must still reach an SVar-named token the way Count$Foretold.X.1 does (the
// brief's Return to Dust / Might of Old Krosa / Haunting Hymn shapes all read
// numbers, so a future SVar spelling must not silently read 0).
func TestIfCastInOwnMainPhaseHeadResolvesSVariantOperands(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"X": "Count$xPaid"}
	c.X = 4
	h.g.Step = state.StepMain1
	h.g.Active = 0
	h.wasCast = true
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.X.2"); got != 4 {
		t.Errorf("own-main IfCastInOwnMainPhase.X.2 = %d, want the SVar X's 4", got)
	}
	h.wasCast = false
	if got := EvalCount(h, c, "Count$IfCastInOwnMainPhase.X.2"); got != 2 {
		t.Errorf("uncast IfCastInOwnMainPhase.X.2 = %d, want the not-main 2", got)
	}
}
