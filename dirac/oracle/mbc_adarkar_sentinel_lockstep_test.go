package oracle

import "testing"

// CR 104.3a, 704.5a: at 1 life, Adarkar Sentinel's "{1}: +0/+1" has only a
// Nomad Stadium ("{T}: Add {W}. Nomad Stadium deals 1 damage to you.") and a
// Boros Signet ("{1}, {T}: Add {R}{W}") to pay with. The Signet is a census
// gap: gorge's prefix search funds its {1} fee by tapping another source, and
// a tap whose own damage kills the payer ends the game, which ends that line
// of the search (scriptSearch.visit's G.Over), so no script pays the play and
// gorge leaves it census_incomplete. The kernel's filter search let the
// Stadium's tap kill the payer and went on, so it offered the activation
// (tail-55-c002 pair 9, "options options"). Its search now ends a line whose
// step takes the payer's life to zero too.
func TestMBCAdarkarSentinelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_search_pain_lethal_funding_tap")
}
