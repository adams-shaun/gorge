package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredAttackerCountBindsSingleAttacker pins that the singular
// TriggeredAttacker value head prefers the bound TriggerCard role over the
// whole Remembered batch a DeclareAttackers trigger captures, with the
// role-absent Remembered read kept as the fallback.
func TestTriggeredAttackerCountBindsSingleAttacker(t *testing.T) {
	t.Parallel()
	h := newHost(t, 2)
	big := h.g.AddObject(mkCard(t, "Name:Big\nManaCost:5\nTypes:Creature Giant\nPT:5/5\nOracle:x\n"), 0)
	small := h.g.AddObject(mkCard(t, "Name:Small\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	if big.ID == small.ID {
		t.Fatalf("precondition: both creatures got the same id %d", big.ID)
	}
	if p := h.g.Obj(big.ID).Face().Power(); p != 5 {
		t.Fatalf("precondition: big power = %d, want 5", p)
	}
	if p := h.g.Obj(small.ID).Face().Power(); p != 2 {
		t.Fatalf("precondition: small power = %d, want 2", p)
	}

	c := &Ctx{TriggerContext: TriggerContext{TriggerCard: big.ID}, Controller: 0,
		Remembered: []state.Target{{Obj: big.ID}, {Obj: small.ID}}}
	if got := EvalCount(h, c, "TriggeredAttacker$CardPower"); got != 5 {
		t.Errorf("TriggeredAttacker$CardPower with TriggerCard bound = %d, want 5 (not the 7-power batch)", got)
	}

	c.TriggerCard = 0
	if got := EvalCount(h, c, "TriggeredAttacker$CardPower"); got != 7 {
		t.Errorf("TriggeredAttacker$CardPower with TriggerCard absent = %d, want 7 (Remembered fallback)", got)
	}
}
