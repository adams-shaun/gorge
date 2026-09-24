package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestCopySpellAbilityRememberCopiesAppendsMintedCopy(t *testing.T) {
	h := newHost(t, 2)
	spell := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	prior := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	ctx := &Ctx{Source: spell.ID, Controller: 0, Remembered: []state.Target{{Obj: prior.ID}}}
	if spell.Zone != state.ZStack || prior.ID == spell.ID {
		t.Fatalf("precondition: source zone=%s, remembered id=%d source=%d", spell.Zone, prior.ID, spell.ID)
	}
	Resolve(h, ctx, sa(t, "DB$ CopySpellAbility | Defined$ Parent | RememberCopies$ True"))
	if got := copyEvents(h); got != 1 {
		t.Fatalf("StackCopy events=%d, want 1", got)
	}
	var copyID state.ObjID
	for _, ob := range h.g.Objs {
		if ob.IsCopy {
			copyID = ob.ID
		}
	}
	if copyID == 0 || copyID == prior.ID || copyID == spell.ID {
		t.Fatalf("minted copy id=%d, prior=%d source=%d", copyID, prior.ID, spell.ID)
	}
	foundPrior, foundCopy := false, false
	for _, target := range ctx.Remembered {
		foundPrior = foundPrior || target.Obj == prior.ID
		foundCopy = foundCopy || target.Obj == copyID
	}
	if !foundPrior || !foundCopy || len(ctx.Remembered) != 2 {
		t.Fatalf("Remembered=%+v, want prior %d and appended copy %d", ctx.Remembered, prior.ID, copyID)
	}
}
