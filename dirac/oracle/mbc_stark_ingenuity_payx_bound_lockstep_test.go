package oracle

import "testing"

// Stark's Ingenuity's "you may pay {X}" enters-the-battlefield trigger asks X
// over what the payer can actually fund (CR 106.6). Gorge bounds its
// announcement by the payment-aware potential; the kernel's plain over-bound
// priced the production open in every colour and offered X to its 64 ceiling
// (decision 463 of the tail-65 focus game: 0..15 on gorge against 0..64 in the
// kernel). The bound lives in dirac/trig_maypayx.go execTrigX.
func TestMBCStarkSIngenuityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stark_ingenuity_payx_bound")
}
