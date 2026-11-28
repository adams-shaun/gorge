package kgen

import (
	"strings"
	"testing"
)

// validStackProbe is a SpellCast copy trigger whose body is Ulalek, Fused
// Atrocity's two CopySpellAbility SVars with the spell spec, the ability spec
// and the trigger line swapped in.
func validStackProbe(trig, spellSpec, abSpec, may string) string {
	return "Name:Sweep Probe\nManaCost:C C\nTypes:Creature Eldrazi\nPT:2/2\n" +
		trig + " | Execute$ TrigCopySpell | TriggerZones$ Battlefield | TriggerDescription$ copy all.\n" +
		"SVar:TrigCopySpell:AB$ CopySpellAbility | Cost$ C C | Defined$ ValidStack " + spellSpec + may + " | SubAbility$ TrigCopyAbilities\n" +
		"SVar:TrigCopyAbilities:DB$ CopySpellAbility | Defined$ ValidStack " + abSpec + may + "\n" +
		"Oracle:copy all.\n"
}

const (
	validStackCastTrig = "T:Mode$ SpellCast | ValidCard$ Card.Eldrazi | ValidActivatingPlayer$ You"
	validStackMay      = " | MayChooseTarget$ True"
)

// TestCopyValidStackForms: Ulalek's two ValidStack copy sweeps lower to two
// CopyStackValid ops (C bit 0 the MayChooseTarget$ election, C bit 1 the
// ability sweep); any other spec, a missing MayChooseTarget$ and a trigger that
// is not SpellCast refuse by name.
func TestCopyValidStackForms(t *testing.T) {
	c := lowerScriptCard(t, "Sweep Probe", validStackProbe(validStackCastTrig, "Spell.YouCtrl", "Ability.YouCtrl+otherAbility", validStackMay))
	for _, rf := range c.Refusals {
		t.Errorf("accepted form refused: %s: %s", rf.Node, rf.Reason)
	}
	var cs []int
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "CopyStackValid" {
				cs = append(cs, in.C)
			}
		}
	}
	if len(cs) != 2 || cs[0] != 1 || cs[1] != 3 {
		t.Errorf("CopyStackValid C bits = %v, want [1 3] (spell sweep, then ability sweep)", cs)
	}

	for _, tc := range []struct {
		name, src, refused string
	}{
		{"opponent's spells", validStackProbe(validStackCastTrig, "Spell.OppCtrl", "Ability.YouCtrl+otherAbility", validStackMay), `ValidStack "Spell.OppCtrl"`},
		{"no MayChooseTarget$", validStackProbe(validStackCastTrig, "Spell.YouCtrl", "Ability.YouCtrl+otherAbility", ""), "ValidStack without MayChooseTarget$ True"},
		{"not a SpellCast trigger", validStackProbe("T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self", "Spell.YouCtrl", "Ability.YouCtrl+otherAbility", validStackMay), "ValidStack outside a SpellCast trigger"},
	} {
		got := strings.Join(lowerScript(t, "Sweep Probe", tc.src), "; ")
		if !strings.Contains(got, tc.refused) {
			t.Errorf("%s: want refusal %q, got %q", tc.name, tc.refused, got)
		}
	}
}
