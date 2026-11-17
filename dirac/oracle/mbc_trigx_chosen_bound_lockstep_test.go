package oracle

import "testing"

// Decree of Justice's cycling trigger ("you may pay {X}") asks its X with a
// chosen-colour source (Sea Gate) untapped on the payer's board. Gorge bounds
// the ask by the payment-aware potential pool, which prices a fixed chosen
// colour as exactly one mana; the kernel read the plain over-bound, where a
// chosen-colour source is open, and offered X = 0..64 against gorge's 0..3
// (decision 430 of 430, dr-mbc-bbaa pair 115).
func TestMBCCrashingDrawbridgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigx_chosen_bound")
}
