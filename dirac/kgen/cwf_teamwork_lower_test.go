package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfFlashTeamworkLowers: Quantum Reduction's `ValidSA$ Spell.Teamwork`
// self grant lowers (CardDef.SelfFlashSA = kop.SfsaTeamwork), while the other
// Spell.* shapes still lower as the plain Spell grant (only Spell.Teamwork sets
// SfsaTeamwork).
func TestSelfFlashTeamworkLowers(t *testing.T) {
	c := lowerNamedCard(t, "Quantum Reduction")
	if len(c.Refusals) != 0 {
		t.Errorf("Quantum Reduction refusals = %v, want none", c.Refusals)
	}
	if !c.SelfFlash || c.SelfFlashSA != kop.SfsaTeamwork {
		t.Errorf("Quantum Reduction SelfFlash=%v SelfFlashSA=%d, want the Teamwork self grant", c.SelfFlash, c.SelfFlashSA)
	}
	// the bare Spell self grant keeps the unconstrained value
	if b := lowerNamedCard(t, "Illusion Spinners"); !b.SelfFlash || b.SelfFlashSA != kop.SfsaSpell {
		t.Errorf("Illusion Spinners SelfFlash=%v SelfFlashSA=%d, want the bare Spell grant", b.SelfFlash, b.SelfFlashSA)
	}
	// the other supported Spell.* shapes lower with the plain Spell grant:
	// the Teamwork SA bit is Quantum Reduction's alone
	for _, name := range []string{"Blur of Heroism", "Flash Photography"} {
		if got := lowerNamedCard(t, name); got.SelfFlash && got.SelfFlashSA != kop.SfsaSpell {
			t.Errorf("%s SelfFlashSA=%d, want the plain Spell grant", name, got.SelfFlashSA)
		}
	}
}
