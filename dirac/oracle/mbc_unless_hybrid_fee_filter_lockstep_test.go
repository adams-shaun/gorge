package oracle

import "testing"

// CR 107.4b with CR 605.3b and CR 601.2g: a filter land's hybrid mana fee
// ("{W/B}, {T}: Add {W}{W}, {W}{B}, or {B}{B}") is paid from the floating mana
// an earlier same-window activation produced, so a payer with an untapped Swamp
// and Fetid Heath can pay Minion of Tevesh Szat's "deals 2 damage to you unless
// you pay {B}{B}": the Swamp's {B} pays the hybrid pip and the Heath makes
// {B}{B}. gorge's unless census (unlessFeeAlts) priced only generic and plain
// coloured fees, so it offered Decline alone while the kernel offered Pay beside
// it (decision 344 of 344; kfuzz tail-01-c000 pair 32 before the fix).
func TestMBCTail01ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_filter_land_hybrid_fee_pays")
}
