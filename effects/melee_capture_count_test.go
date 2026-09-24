package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestMeleePumpCountReadsTheCaptureOnPurpose pins the Melee half of the
// capture exclusion: rules seeds a Melee trigger with one player ref per
// attacked opponent as BOTH Remembered and Captured, so the plain
// Count$RememberedNumber head (which excludes the capture) reads 0 there and
// the synthesized pump must read the capture through cards.MeleePumpCount.
// The object entries and the plain Remembered set must not leak into it.
func TestMeleePumpCountReadsTheCaptureOnPurpose(t *testing.T) {
	h := newHost(t, 3)
	src := mkCard(t, "Name:Src\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n")
	so := h.g.AddObject(src, 0)
	so.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), so.ID))

	players := []state.Target{{Player: 1, IsPlayer: true}, {Player: 2, IsPlayer: true}}
	c := &Ctx{Source: so.ID, Controller: 0,
		Remembered: append([]state.Target(nil), players...),
		Captured:   append([]state.Target(nil), players...)}
	if n, ok := EvalCountOK(h, c, "Count$RememberedNumber"); !ok || n != 0 {
		t.Fatalf("precondition: Count$RememberedNumber over a pure capture = %d ok=%v, want 0 true", n, ok)
	}
	if n, ok := EvalCountOK(h, c, cards.MeleePumpCount); !ok || n != 2 {
		t.Fatalf("%s = %d ok=%v, want 2 true (two attacked opponents)", cards.MeleePumpCount, n, ok)
	}

	// Only the capture's PLAYER entries count; a remembered-but-uncaptured
	// player (something the body itself remembered) does not.
	mixed := &Ctx{Source: so.ID, Controller: 0,
		Remembered: []state.Target{{Obj: so.ID}, {Player: 1, IsPlayer: true}, {Player: 2, IsPlayer: true}},
		Captured:   []state.Target{{Obj: so.ID}, {Player: 1, IsPlayer: true}}}
	if n, ok := EvalCountOK(h, mixed, cards.MeleePumpCount); !ok || n != 1 {
		t.Fatalf("mixed capture %s = %d ok=%v, want 1 true", cards.MeleePumpCount, n, ok)
	}

	// No capture (a non-trigger ctx) reads zero, and a non-Amount property
	// fails closed.
	if n, ok := EvalCountOK(h, &Ctx{Source: so.ID, Remembered: players}, cards.MeleePumpCount); !ok || n != 0 {
		t.Fatalf("no-capture %s = %d ok=%v, want 0 true", cards.MeleePumpCount, n, ok)
	}
	if _, ok := EvalCountOK(h, c, "TriggeredCapturedPlayers$LifeTotal"); ok {
		t.Fatalf("TriggeredCapturedPlayers$LifeTotal evaluated; only Amount is modelled")
	}
}
