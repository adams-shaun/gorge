package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCountTotalCommanderCastFromCommandZoneHead pins the
// Count$TotalCommanderCastFromCommandZone head through the Host seam (the
// fake double; the real log-walk read is pinned in rules' commander cast
// count test on the real Thunderclap Drake). The head must:
//
//   - report the resolving controller's own per-player count (the head's
//     "you've cast" scoping lives in the Host read, keyed by c.Controller),
//   - accept the /Twice word op (The Swarmlord's
//     Count$TotalCommanderCastFromCommandZone/Twice), and
//   - report 0 for a controller the double has no entry for (a nil map
//     answers zero, the pre-existing conservative no-op) — but still as a
//     RESOLVED zero, never the unresolvable (0, false) verdict, since the
//     head itself always resolves once registered.
func TestCountTotalCommanderCastFromCommandZoneHead(t *testing.T) {
	h, base := fixtureHost(t)
	h.commanderCasts = map[state.PlayerID]int32{0: 3, 1: 1}
	c := *base

	if got, ok := EvalCountOK(h, &c, "Count$TotalCommanderCastFromCommandZone"); !ok || got != 3 {
		t.Errorf("seat 0 count = (%d, %v), want (3, true)", got, ok)
	}

	c1 := *base
	c1.Controller = 1
	if got, ok := EvalCountOK(h, &c1, "Count$TotalCommanderCastFromCommandZone"); !ok || got != 1 {
		t.Errorf("seat 1 count = (%d, %v), want (1, true)", got, ok)
	}

	// The Swarmlord's /Twice spelling: the word op rides the generic /Op
	// machinery, so it doubles without a head-specific arm.
	cT := *base
	if got, ok := EvalCountOK(h, &cT, "Count$TotalCommanderCastFromCommandZone/Twice"); !ok || got != 6 {
		t.Errorf("/Twice count = (%d, %v), want (6, true)", got, ok)
	}

	// A controller with no entry answers 0 — a resolved zero, not the
	// unresolvable verdict.
	c2 := *base
	c2.Controller = state.PlayerID(2)
	if got, ok := EvalCountOK(h, &c2, "Count$TotalCommanderCastFromCommandZone"); !ok || got != 0 {
		t.Errorf("seat 2 count = (%d, %v), want (0, true)", got, ok)
	}
}
