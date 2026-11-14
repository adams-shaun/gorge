package kgen

import (
	"strings"
	"testing"
)

// TestMBCCombat6Lowering: the combat-6 spellings lower. Plagon's and
// Outmaneuver's Effect-delivered assignment statics become the kernel's
// AsgnGrant over the target (C 0 toughness, C 1 as though unblocked);
// Karlach's FirstCombat$ True on an AttackersDeclared trigger becomes the
// combats-begun == 1 fire-only condition (the card still refuses wholesale on
// Choose a Background, so only that refusal remains).
func TestMBCCombat6Lowering(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Plagon, Lord of the Beach", "AsgnGrant obj=Target"},
		{"Outmaneuver", "AsgnGrant obj=Target"},
	} {
		c := lowerCorpusCard(t, tc.name)
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s: %s", tc.name, r.Node, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), tc.want) {
			t.Errorf("%s: no %q:\n%s", tc.name, tc.want, sb.String())
		}
	}
	var plagonC, outC int = -1, -1
	for name, dst := range map[string]*int{"Plagon, Lord of the Beach": &plagonC, "Outmaneuver": &outC} {
		for _, a := range lowerCorpusCard(t, name).Abilities {
			for _, in := range a.Prog {
				if in.Op == "AsgnGrant" {
					*dst = in.C
				}
			}
		}
	}
	if plagonC != 0 || outC != 1 {
		t.Errorf("AsgnGrant C: Plagon %d (want 0), Outmaneuver %d (want 1)", plagonC, outC)
	}

	k := lowerCorpusCard(t, "Karlach, Fury of Avernus")
	for _, r := range k.Refusals {
		if !strings.Contains(r.Reason, "Choose a Background") {
			t.Errorf("Karlach refused beyond Choose a Background: %s: %s", r.Node, r.Reason)
		}
	}
	var sb strings.Builder
	dumpLIR(&sb, k)
	if !strings.Contains(sb.String(), "PhaseCount") {
		t.Errorf("Karlach: no PhaseCount (combats begun) condition on the trigger:\n%s", sb.String())
	}
}
