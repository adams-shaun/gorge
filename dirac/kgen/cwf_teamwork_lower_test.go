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
	// a ValidSA$ shape the lowerer does not read still refuses by name. The
	// control is a probe, not a corpus carrier: every real Spell.* carrier
	// lowers now (the IsTargeting one: cwf_istargeting_lower_test.go; the
	// Spell.Hero subtype: cwf_xcost_subtype_lower_test.go).
	src := "Name:Probe\nManaCost:2 W\nTypes:Enchantment\nS:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Spell.Bogus | Caster$ You\nOracle:x\n"
	if got := lowerScript(t, "Probe", src); !slices.Contains(got, "CastWithFlash shape") {
		t.Errorf("ValidSA$ Spell.Bogus refusals = %v, want CastWithFlash shape", got)
	}
}
