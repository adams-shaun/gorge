package kgen

import (
	"strings"
	"testing"
)

// TestEnchantedByEntersRefused: an Aura's self-enters trigger whose
// IsPresent$ reads the Aura's host (Historian's Wisdom, Permanent.EnchantedBy+
// greatestPower; its siblings Earthbind, Howl of the Hunt, Ray of Frost and
// Volition Reins) is refused. gorge checks it with the Aura attached
// (CR 303.4f); the kernel queues the ETB trigger in enter(), before
// resolve.go attaches the Aura, so the filter matches nothing and the trigger
// never fires: TestFilterWordsOtherContextsLockstep diverged on every game that cast
// Historian's Wisdom (dirac/oracle/filterothercontexts_test.go).
func TestEnchantedByEntersRefused(t *testing.T) {
	names := []string{"Historian's Wisdom", "Earthbind", "Howl of the Hunt", "Ray of Frost", "Volition Reins"}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "reads EnchantedBy before the Aura attaches") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no EnchantedBy refusal (refusals %v)", c.Name, c.Refusals)
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d carriers", seen, len(names))
	}
}
