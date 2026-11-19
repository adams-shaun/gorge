package oracle

import "testing"

// CR 603.10a with CR 120.1: Vampiric Embrace grants its enchanted creature
// "whenever a creature dealt damage by this creature this turn dies, put a
// +1/+1 counter on it". The kernel recorded damage marks only for a card whose
// own ability reads DamagedBy, so the Aura's host (Embodiment of Insight) never
// marked the creature it killed in combat and the granted trigger was missing
// from the trigger-order decision (pair 15 of tail-47-c000).
func TestMBCEmbodimentOfInsightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_damagedby_host")
}
