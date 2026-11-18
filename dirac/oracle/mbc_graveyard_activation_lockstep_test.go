package oracle

import "testing"

// CR 307.1, 602.5d: Grafted Butcher's graveyard ability ({3}{B}, sacrifice an
// artifact or creature) is sorcery-speed, and each of p1's eight lands queues
// a Manabarbs trigger when tapped. gorge's prefix search tries every deeper
// activation before the pass that resolves the triggers, so when tapping all
// eight kills p1 (8 life) it falls back to a shallower prefix and still offers
// the activation. The kernel's search judged only the all-gaps leaf and dropped
// the offer (tail-34 autopay pair 4, decision 662).
func TestMBCGraftedButcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sorcery_prefix_fallback_when_full_taps_kill")
}
