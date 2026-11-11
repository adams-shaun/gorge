package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ConditionActivationLimit$ GE<n> on the DB behind a creature's one pump
// ability lowers to a compare of kop.ExActivations; the kernel counts
// activations per object, so a card with a second non-mana activated ability
// (or a mana ability carrying the gate) refuses by name.
const activationCountScript = "Name:Test Whelp\nManaCost:2 R R\nTypes:Creature Dragon\nPT:2/3\nK:Flying\n" +
	"A:AB$ Pump | Cost$ R | Defined$ Self | NumAtt$ +1 | SubAbility$ DBBurn\n" +
	"SVar:DBBurn:DB$ Pump | Defined$ Self | AtEOT$ Sacrifice | ConditionActivationLimit$ GE4\n" +
	"Oracle:x\n"

func TestActivationLimitConditionLowers(t *testing.T) {
	c := lowerScriptCard(t, "Test Whelp", activationCountScript)
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	var found bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			cd := in.Cond
			if in.Op == "SkipIf" && cd.Kind == cValue && cd.A.Kind == vActivations && cd.Op == opGE && cd.B.isConst() && cd.B.N == 4 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no SkipIf on vActivations GE 4 in %+v", c.Abilities)
	}
	if exprKinds[vActivations] != kop.ExActivations {
		t.Fatalf("vActivations builds to %v", exprKinds[vActivations])
	}
}

func TestActivationLimitConditionRefuses(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"second activated ability", strings.Replace(activationCountScript, "Oracle:x", "A:AB$ Draw | Cost$ 2 R | NumCards$ 1\nOracle:x", 1), "non-mana activated abilities"},
		{"beside another gate", strings.Replace(activationCountScript, "ConditionActivationLimit$ GE4", "ConditionActivationLimit$ GE4 | ConditionPlayerTurn$ True", 1), "beside"},
	} {
		r := lowerScript(t, "Test Whelp", tc.src)
		if len(r) == 0 || !strings.Contains(strings.Join(r, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, r, tc.want)
		}
	}
}
