package oracle

import "testing"

// CR 107.4b with CR 601.2h: Fetid Heath's "{W/B}, {T}: Add {W}{W}, {W}{B},
// or {B}{B}" spends one mana and produces two, so a board with a Swamp, a
// Fetid Heath and enough white sources pays Peer into the Abyss's {B}{B}{B}.
// At decision 440 of the recorded game the kernel offered the cast and gorge
// did not: the relaxed payment census did not read the hybrid pip as a fee,
// so the second activation's {B} was never funded and the cast was wrongly
// withheld. The census now reads a hybrid pip's fee, and both engines offer
// the cast.
func TestMBCPeerIntoTheAbyssReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_fee_funds_the_cast")
}
