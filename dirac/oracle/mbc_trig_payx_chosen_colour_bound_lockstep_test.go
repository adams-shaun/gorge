package oracle

import "testing"

// The first divergence of the tail evidence pool dr-mbc-bbaa, pair 127 (seed
// 508209, autopay, gorge leading): a cycled Decree of Justice's "you may pay
// {X}" trigger asks its X beside two Plains and a Sea Gate whose colour is
// already chosen ({T}: Add {U} or one mana of the chosen color). Gorge's
// announcement bound (cumulative.go triggeredCostXAsk, potentialManaFor with
// the trigger cost as the payment) prices that chosen-colour source exactly,
// so X runs 0..4; the kernel's bound (trig_maypayx.go execTrigX, the plain
// over-bound potential) priced the chosen production open in every colour and
// offered X up to its 64 ceiling.
func TestMBCGoldmeadowHarrierReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_payx_chosen_colour_bound")
}
