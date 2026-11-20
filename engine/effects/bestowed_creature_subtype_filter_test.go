package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The bestowed switch's creature-subtype arm. CR 702.103b turns a bestowed
// cast into an Aura enchantment, and CR 205.1a takes the lost Creature card
// type's subtypes with it: the printed "Enchantment Creature Merfolk Wizard"
// line leaves only Enchantment Aura behind. The switch answered Creature=no
// but let every creature SUBTYPE word fall through to the printed face, so a
// Count$Valid Wizard (Ixidor's Will's SVar for its "unless its controller
// pays {2} for each Wizard") counted a bestowed Triton Wavebreaker.

const (
	bestowWizardSrc = "Name:Triton Wavebreaker\nManaCost:U\nTypes:Enchantment Creature Merfolk Wizard\nPT:1/1\nK:Bestow:1 U\nOracle:x\n"
	bestowHostSrc   = "Name:Grizzly Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

func TestBestowedAuraCreatureSubtypesAreGone(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	host := g.AddObject(mkCard(t, bestowHostSrc), 0)
	host.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), host.ID))
	bw := g.AddObject(mkCard(t, bestowWizardSrc), 0)
	bw.Zone = state.ZBattlefield
	bw.AttachedTo = host.ID
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), bw.ID))
	// PRECONDITION: the Triton really is a bestowed Aura attached to the bear
	// (not a Creature: the switch's existing arm), the bear is the only
	// creature you control, and the printed face still says Wizard.
	if !bw.BestowedAttached() {
		t.Fatal("precondition: the Triton is not read as a bestowed Aura")
	}
	if hasType(bw, "Creature") {
		t.Fatal("precondition: the bestowed Aura still answers Creature=yes")
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl"); got != 1 {
		t.Fatalf("Count$Valid Creature.YouCtrl = %d, want 1 (the bear alone)", got)
	}
	if !bw.Face().TypeLineHas("Wizard", 0) {
		t.Fatal("precondition: the printed face's Wizard word is not found at all")
	}
	if got := EvalCount(h, c, "Count$Valid Wizard.YouCtrl"); got != 0 {
		t.Fatalf("Count$Valid Wizard.YouCtrl = %d, want 0: the bestowed Aura's printed Wizard is gone", got)
	}
	if !hasType(bw, "Aura") || !hasType(bw, "Enchantment") {
		t.Fatal("the bestowed Aura keeps its Aura and Enchantment types")
	}
	if hasType(bw, "Merfolk") {
		t.Fatal("the bestowed Aura's printed Merfolk subtype is gone too")
	}
}
