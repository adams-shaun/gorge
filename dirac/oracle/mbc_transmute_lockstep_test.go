package oracle

import "testing"

// CR 702.53a: Transmute is a sorcery-speed activation from the hand ({1}{B}{B},
// discard this card). Netherborn Phalanx's was offered by gorge and not by the
// kernel at decision 236 of tail-36 pair 11.
func TestMBCNetherbornPhalanxReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "transmute_hand_activation_offered")
}
