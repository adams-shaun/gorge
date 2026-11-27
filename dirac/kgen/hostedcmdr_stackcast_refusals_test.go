package kgen

import (
	"strings"
	"testing"
)

// TestHostedCmdrStackCastRefusals pins the hosted-Commander stack and cast
// cards by name: Springheart Nantuko (a node's own ConditionPresent$ guard over
// an UnlessResolveSubs$ gate) lowers; the cards whose shape needs a kernel
// mechanism that does not exist yet keep refusing, each with its own reason:
//
//   - Commandeer, Disrupting Shoal and Blazing Shoal now lower (their
//     ControlSpell, Counter Defined$ Targeted and Announce$ X shapes landed,
//     cli-20261127T171908Z): pinned empty here, and by
//     TestHostedCmdrStackCastLowers and the dirac/oracle lockstep tests;
//   - Vivi Ornitier: a PlayerTurn$ gate on a mana ability (lowering it passes
//     this check, then the kernel's mana gate refuses: it gates only a plain {T}
//     amount-1 unrestricted ability, not Vivi's {0} X-amount choice);
//   - Venerated Loxodon: Defined$ Convoked (the kernel keeps only the count);
//   - Ob Nixilis, Captive Kingpin: LifeLostAll (a simultaneous-loss batch the
//     kernel does not form; kop.TrigLifeLostAll is reserved).
//
// Wastescape Battlemage's refusal sat at the build step (selfCastClause), past
// this pass; its lowering is dirac.TestWastescapeKickedCastTriggers's.
func TestHostedCmdrStackCastRefusals(t *testing.T) {
	cs := lowerNamed(t, []string{
		"Springheart Nantuko", "Commandeer", "Disrupting Shoal", "Blazing Shoal", "Vivi Ornitier",
		"Venerated Loxodon", "Ob Nixilis, Captive Kingpin",
	})
	want := map[string]string{
		"Springheart Nantuko":         "",
		"Commandeer":                  "",
		"Disrupting Shoal":            "",
		"Blazing Shoal":               "",
		"Vivi Ornitier":               `activation PlayerTurn$ "True" beside another condition`,
		"Venerated Loxodon":           `Defined$ "Convoked"`,
		"Ob Nixilis, Captive Kingpin": "trigger mode LifeLostAll",
	}
	seen := 0
	for _, c := range cs {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		if w == "" && got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
		}
		if w != "" && !strings.Contains(got, w) {
			t.Errorf("%s refusal = %q, want it to contain %q", c.Name, got, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}
