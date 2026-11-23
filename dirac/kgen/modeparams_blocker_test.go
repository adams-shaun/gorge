package kgen

import (
	"strings"
	"testing"
)

// TestModeParamBlockerScopeLowers: an AttackerBlocked line's ValidBlocker$
// other than Card.Self lowers to the second filter (the kernel's blocker
// filter), one census card per spelling.
func TestModeParamBlockerScopeLowers(t *testing.T) {
	names := []string{"Tel-Jilad Wolf", "Serra Inquisitors", "Hedron Blade", "Shield of the Righteous", "Amphibious Kavu", "Dwarven Soldier"}
	for _, c := range lowerNamed(t, names) {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		blocked := 0
		for _, tr := range trigsOf(c, "AttackerBlocked") {
			blocked++
			if tr.F2 == nil || tr.BlockedBySelf {
				t.Errorf("%s: AttackerBlocked line %+v, want a blocker filter and no BlockedBySelf", c.Name, tr)
			}
		}
		if blocked == 0 {
			t.Errorf("%s: no AttackerBlocked line lowered", c.Name)
		}
	}
}

// TestModeParamBlockerScopeRefuses: ValidBlockerAmount$ (Seifer, Balamb
// Rival) is a count over the matching blockers the kernel does not read, so
// it still refuses by name.
func TestModeParamBlockerScopeRefuses(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Seifer, Balamb Rival"}) {
		if len(c.Refusals) == 0 || !strings.Contains(c.Refusals[0].Reason, "ValidBlockerAmount") {
			t.Errorf("%s: refusals %v, want one naming ValidBlockerAmount$", c.Name, c.Refusals)
		}
	}
}
