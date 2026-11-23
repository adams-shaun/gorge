package oracle

import "testing"

// Resplendent Marshal's "you may exile another creature card from your
// graveyard" must not offer the Marshal itself once it lies in its own
// graveyard (gorge's Other: not the source's own object). dirac's game-wide
// graveyard pick matched the filter against the card alone and offered the
// dying source's card beside the other candidates (decision 602 of tail-20
// pair 13, seed 980429, autopay).
func TestMBCResplendentMarshalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pick_other_source_offered")
}
