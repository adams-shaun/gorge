package oracle

import "testing"

// The elided-mana mirror (elided.go): gorge's engine funds a play's payment
// plan's mana activations inside its own answer processing, where the canon
// stream carries nothing, and prices its next menu from that position; the
// kernel re-derives the same payment at the answer's Apply. A resolution-time
// cost payment (the Citanul Centaurs echo this game elects to pay) is
// different: gorge funds it at its Step, after the answer's Apply, so the
// kernel's own re-derivation at that Apply has already landed on gorge's pool
// by the time the step drain reads. Re-applying the plan's uses doubled the
// floats (Mountain#22 tapped for R, Overflowing Basin's {1} fee spent,
// {G}{U} floated twice: kernel Pool[0][1]/[0][4] 2 against gorge's 1) and the
// lockstep diverged at decision 803; the step drain now drops the uses of a
// seat whose pool the kernel already holds at gorge's position (the original
// over-offer of cast Thunderscape Master at decision 897, the kernel's
// pre-activation menu, is the same game's first instance).
func TestMBCThunderscapeMasterElidedPayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "thunderscape_master_elided_pay_reconciled")
}
