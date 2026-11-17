package oracle

import "testing"

// CR 605.3a, 601.2g: two untapped Plains and two Celestial Prisms ("{2}, {T}:
// Add one mana of any color") pay Hard Evidence's {U}: two Plains fund one
// Prism's fee and it makes the {U}. gorge's relaxed proof runs each subset of
// the census's paid filters, so the second Prism, whose fee the board cannot
// spare, did not hide the play; the kernel charged every filter at once and
// left the cast out (decision 1502 of tail-30 pair 4, full surface). The
// recording plays its decks on a table built from them (pool custom): the
// synthetic pool tail-30 is regenerated with the tree.
func TestMBCHardEvidenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hard_evidence_cast_offer_lockstep")
}
