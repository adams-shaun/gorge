package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfFlashTeamworkLowers: Quantum Reduction's `ValidSA$ Spell.Teamwork`
// self grant lowers (CardDef.SelfFlashSA = kop.SfsaTeamwork), while every other
// ValidSA$ shape stays refused with the same "CastWithFlash shape".
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
	// a different Spell.* carrier still refuses by name
	for _, name := range []string{"Blur of Heroism", "Flash Photography"} {
		got := lowerNamedCard(t, name)
		if !slices.ContainsFunc(got.Refusals, func(r refusal) bool { return r.Reason == "CastWithFlash shape" }) {
			t.Errorf("%s refusals = %v, want CastWithFlash shape", name, got.Refusals)
		}
	}
}
