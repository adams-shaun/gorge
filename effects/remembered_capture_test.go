package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestRememberedCaptureExcludedFromPlainReads pins the capture exclusion every
// plain `Remembered$...` read now applies (task rememberedcapture1).
//
// rules seeds a firing trigger's execution ctx with Remembered == Captured ==
// the trigger's fire-time event capture. Forge's host remembered list never
// contains the event object -- the trigger body reads it through the separate
// Triggered* family -- so a raw len(Ctx.Remembered) or a raw group walk
// counts the capture as something the resolution itself remembered. The
// measured symptoms: evalRememberedOK's Amount head read 1 for a phase
// trigger body that remembered nothing (Enchanter's Bane's "did this
// resolution sacrifice nothing?" EQ0 gate could never hold), and a
// Remembered$Valid read admitted the event object itself for an
// event-object trigger (capture = ev.Obj != source, which the source-skip in
// rememberedWithSource does NOT mask).
//
// Each sub-case asserts its own precondition -- the capture object is really
// the source (phase shape) or really distinct from it (event shape), and the
// real remembered member really has the property read -- so a vacuous fixture
// fails loudly instead of passing on an empty list.
func TestRememberedCaptureExcludedFromPlainReads(t *testing.T) {
	h := newHost(t, 2)
	// A creature, so Remembered$Valid Creature has a matching spec AND a
	// non-matching one, and a distinct mana value for the property head.
	beast := mkCard(t, "Name:Beast\nManaCost:3\nTypes:Creature Beast\nPT:3/4\nOracle:x\n")
	// The trigger's source: a creature too, so a filtered read over the
	// capture would MATCH if the exclusion were absent.
	src := mkCard(t, "Name:Src\nManaCost:5\nTypes:Creature\nPT:2/2\nOracle:x\n")
	// A non-creature with a different mana value, for the Valid-contrast and
	// property cases.
	rock := mkCard(t, "Name:Rock\nManaCost:7\nTypes:Artifact\nOracle:x\n")
	bo := h.g.AddObject(beast, 0)
	so := h.g.AddObject(src, 0)
	ro := h.g.AddObject(rock, 0)
	for _, id := range []state.ObjID{bo.ID, so.ID, ro.ID} {
		h.g.Obj(id).Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), id))
	}

	// --- phase-trigger shape: capture == source, real remembered set empty.
	phaseCtx := &Ctx{
		Source:     so.ID,
		Controller: 0,
		Remembered: []state.Target{{Obj: so.ID}},
		Captured:   []state.Target{{Obj: so.ID}},
	}
	if got := h.g.Obj(so.ID); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source not on the battlefield")
	}
	if len(phaseCtx.Remembered) != 1 || phaseCtx.Remembered[0] != phaseCtx.Captured[0] {
		t.Fatalf("precondition: phase ctx not seeded Remembered==Captured==source")
	}
	if n, ok := EvalCountOK(h, phaseCtx, "Remembered$Amount"); !ok || n != 0 {
		t.Fatalf("phase-capture Remembered$Amount = %d ok=%v, want 0 true (capture must not count)", n, ok)
	}

	// --- event-object shape: capture = the event object, distinct from the
	// source; the remembered set holds only the capture. Both the Valid head
	// and the group gate must read empty.
	eventCtx := &Ctx{
		Source:     so.ID,
		Controller: 0,
		Remembered: []state.Target{{Obj: bo.ID}},
		Captured:   []state.Target{{Obj: bo.ID}},
	}
	if bo.ID == so.ID {
		t.Fatalf("precondition: event capture coincides with the source")
	}
	if o := h.g.Obj(bo.ID); o == nil || !o.Face().IsCreature() {
		t.Fatalf("precondition: event capture is not a Creature (Valid Creature would deny it anyway)")
	}
	if n, ok := EvalCountOK(h, eventCtx, "Remembered$Valid Creature"); !ok || n != 0 {
		t.Fatalf("event-capture Remembered$Valid Creature = %d ok=%v, want 0 true", n, ok)
	}
	// The group gate (ConditionDefined$ Remembered) must agree with the count.
	gate := sa(t, "DB$ Pump | ConditionDefined$ Remembered | ConditionCompare$ EQ0")
	if met, resolved := conditionMet(h, eventCtx, gate); !resolved || !met {
		t.Fatalf("ConditionDefined$ Remembered EQ0 over capture-only ctx: met=%v resolved=%v, want true true", met, resolved)
	}
	// And the presence direction must deny (the capture is not remembered).
	presence := sa(t, "DB$ Pump | ConditionDefined$ Remembered | ConditionPresent$ Card")
	if met, resolved := conditionMet(h, eventCtx, presence); !resolved || met {
		t.Fatalf("ConditionPresent$ Card over capture-only ctx: met=%v resolved=%v, want false true", met, resolved)
	}

	// --- property head unchanged modulo the capture: with the capture PLUS a
	// real remembered artifact, Remembered$CardManaCost sums only the artifact.
	propCtx := &Ctx{
		Source:     so.ID,
		Controller: 0,
		Remembered: []state.Target{{Obj: bo.ID}, {Obj: ro.ID}},
		Captured:   []state.Target{{Obj: bo.ID}},
	}
	if o := h.g.Obj(ro.ID); o == nil || !o.Face().IsArtifact() || o.Face().ManaValue() != 7 {
		t.Fatalf("precondition: rock is not an artifact with mana value 7 (property read would be vacuous)")
	}
	if n, ok := EvalCountOK(h, propCtx, "Remembered$CardManaCost"); !ok || n != 7 {
		t.Fatalf("Remembered$CardManaCost = %d ok=%v, want 7 true (rock only, capture excluded)", n, ok)
	}

	// --- no-capture ctx unchanged: a plain spell resolution counts every
	// remembered object. The two real members sum to 10.
	spellCtx := &Ctx{
		Source:     so.ID,
		Controller: 0,
		Remembered: []state.Target{{Obj: bo.ID}, {Obj: ro.ID}},
	}
	if n, ok := EvalCountOK(h, spellCtx, "Remembered$Amount"); !ok || n != 2 {
		t.Fatalf("no-capture Remembered$Amount = %d ok=%v, want 2 true (unchanged)", n, ok)
	}
	if n, ok := EvalCountOK(h, spellCtx, "Remembered$CardManaCost"); !ok || n != 10 {
		t.Fatalf("no-capture Remembered$CardManaCost = %d ok=%v, want 10 true (unchanged)", n, ok)
	}
	// A no-capture event-less ctx must also still admit its members for Valid.
	if n, ok := EvalCountOK(h, spellCtx, "Remembered$Valid Creature"); !ok || n != 1 {
		t.Fatalf("no-capture Remembered$Valid Creature = %d ok=%v, want 1 true (beast only)", n, ok)
	}
}
