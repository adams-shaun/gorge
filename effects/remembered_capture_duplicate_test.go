package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// A resolving body can remember the same object that its trigger captured.
// Exclusion removes the seeded occurrence, not the later memory occurrence.
func TestRememberedCaptureExclusionPreservesExplicitSameObject(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Creature\nManaCost:3\nTypes:Creature\nPT:2/2\nOracle:x\n")
	sourceCard := mkCard(t, "Name:Source\nManaCost:1\nTypes:Enchantment\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	source := h.g.AddObject(sourceCard, 0)
	o.Zone = state.ZBattlefield
	source.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID, source.ID})
	capture := state.Target{Obj: o.ID}
	c := &Ctx{Source: source.ID, Controller: 0,
		Remembered: []state.Target{capture, capture}, Captured: []state.Target{capture}}
	if o.Zone != state.ZBattlefield || c.Remembered[0] != c.Captured[0] || c.Remembered[1] != c.Captured[0] {
		t.Fatal("precondition: trigger capture and explicit remembered occurrence must identify the same battlefield object")
	}
	if n, ok := EvalCountOK(h, c, "Remembered$Amount"); !ok || n != 1 {
		t.Fatalf("Remembered$Amount = %d ok=%v, want 1 explicit memory", n, ok)
	}
	if n, ok := EvalCountOK(h, c, "Remembered$Valid Creature"); !ok || n != 1 {
		t.Fatalf("Remembered$Valid Creature = %d ok=%v, want the explicitly remembered creature", n, ok)
	}
}
