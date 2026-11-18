package oracle

import "testing"

// Bedeck ({B/R}{B/R} instant half of Bedeck // Bedazzle) cast mid-payment:
// gorge and dirac offered different payment choices at decision 234 of the
// recorded tail-15-c000 game (autopay, mulligans 1, seed 210986).
func TestMBCBedeckReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bedeck_hybrid_payment_options")
}
