package kgen

import (
	"strings"
	"testing"
)

// TestDigUntilBattlefieldRememberRefusedByName: DigUntil's RememberRevealed$
// carriers whose found card enters the battlefield (Madcap Experiment,
// Audacious Reshapers) stay refused with the named found-zBattlefield reason
// (the lowering never reads the found card entering the battlefield; the
// once-suspected sac-source prefix spin in resolvePrefixStack is fixed), not
// a bare "unread params" or a cascaded cross-ability read; Spoils of the
// Vault, the found-zHand carrier, still lowers.
func TestDigUntilBattlefieldRememberRefusedByName(t *testing.T) {
	names := []string{"Madcap Experiment", "Audacious Reshapers", "Spoils of the Vault"}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, names) {
		seen[c.Name] = true
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		got := strings.Join(all, "; ")
		if c.Name == "Spoils of the Vault" {
			if len(c.Refusals) != 0 {
				t.Errorf("%s: refusals %q, want none", c.Name, got)
			}
			continue
		}
		if !strings.Contains(got, "DigUntil RememberRevealed$ with the found card entering the battlefield") {
			t.Errorf("%s: refusals %q, want the named found-zBattlefield reason", c.Name, got)
		}
		if strings.Contains(got, "unread params") || strings.Contains(got, "reads Remembered written by another ability") {
			t.Errorf("%s: refused as a cascaded/unnamed blocker, not by name: %q", c.Name, got)
		}
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
