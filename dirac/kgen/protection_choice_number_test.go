package kgen

import (
	"strings"
	"testing"
)

// TestProtectionChoiceRefusesWithChooseNumber: the kernel's protection
// answer and a mid-resolution ChooseNumber answer share res.bad bit 0x20 and
// res.chosen (dirac resProtChoice / resChosenNumber), so a resolution that
// carries both asks, in either order, is refused rather than lowered into a
// program whose second answer overwrites the first.
func TestProtectionChoiceRefusesWithChooseNumber(t *testing.T) {
	prot := "Protection | ValidTgts$ Creature | TgtPrompt$ Select target creature | Gains$ Choice | Choices$ AnyColor,colorless"
	cases := map[string]string{
		"number then protection": "A:SP$ ChooseNumber | Min$ 1 | Max$ 3 | SubAbility$ DBProt\nSVar:DBProt:DB$ " + prot + "\n",
		"protection then number": "A:SP$ " + prot + " | SubAbility$ DBNum\nSVar:DBNum:DB$ ChooseNumber | Min$ 1 | Max$ 3\n",
	}
	for name, body := range cases {
		c := lowerScriptCard(t, "Probe Prot Number", "Name:Probe Prot Number\nManaCost:1 W\nTypes:Instant\n"+body+"Oracle:x\n")
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Reason)
		}
		if !strings.Contains(strings.Join(got, "; "), "in one resolution") {
			t.Errorf("%s: want the shared-answer refusal, got %q", name, got)
		}
	}
	// control: the protection choice alone lowers
	c := lowerScriptCard(t, "Probe Prot Alone", "Name:Probe Prot Alone\nManaCost:1 W\nTypes:Instant\n"+
		"A:SP$ Protection | ValidTgts$ Creature | TgtPrompt$ Select target creature | Gains$ Choice | Choices$ AnyColor,colorless\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("control refused: %v", c.Refusals)
	}
}
