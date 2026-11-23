package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// rippleSimultaneousTriggers counts the ripple triggers the cast put on the
// stack. castFixture stops at the first non-priority ask: two or more
// simultaneous triggers pose the CR 603.3b ordering ask (one option each); a
// lone trigger goes straight to its resolution-time may.
func rippleSimultaneousTriggers(t *testing.T, e *Engine) int {
	t.Helper()
	d := e.Pending()
	switch {
	case d == nil:
		t.Fatal("no decision pending after the cast")
	case d.Kind == decision.KTriggerOrder:
		return len(d.Options)
	case d.Kind == decision.KTriggerOptional:
		return 1
	}
	t.Fatalf("unexpected %v ask after the cast: %+v", d.Kind, d)
	return 0
}

// TestThrummingStonePlusPrintedRippleTriggersTwice is CR 702.60b: each
// instance of ripple triggers separately, so a spell that prints ripple and is
// granted it too (Surging Flame under Thrumming Stone) puts two ripple
// triggers on the stack, while a spell that only has the granted one (Relentless
// Rats) or only the printed one puts one.
func TestThrummingStonePlusPrintedRippleTriggersTwice(t *testing.T) {
	t.Parallel()
	window := []string{"Mountain", "Grizzly Bears", "Mountain", "Grizzly Bears"}

	e, _, flameID, _ := rippleTestEngine(t, 77106, "Surging Flame", window, []string{"Thrumming Stone"})
	addMana(t, e, 0, "RR")
	castFixture(t, e, flameID, 1)
	if n := rippleSimultaneousTriggers(t, e); n != 2 {
		t.Fatalf("Surging Flame + Thrumming Stone: %d ripple triggers, want 2 (printed + granted)", n)
	}

	e, _, ratsID, _ := rippleTestEngine(t, 77107, "Relentless Rats", window, []string{"Thrumming Stone"})
	addMana(t, e, 0, "BBB")
	castFixture(t, e, ratsID, -1)
	if n := rippleSimultaneousTriggers(t, e); n != 1 {
		t.Fatalf("Relentless Rats + Thrumming Stone: %d ripple triggers, want 1 (granted only)", n)
	}

	// A printed carrier with no grant is still exactly one instance.
	e, _, flameID, _ = rippleTestEngine(t, 77108, "Surging Flame", window, nil)
	addMana(t, e, 0, "RR")
	castFixture(t, e, flameID, 1)
	if n := rippleSimultaneousTriggers(t, e); n != 1 {
		t.Fatalf("Surging Flame alone: %d ripple triggers, want 1", n)
	}
}
