package oracle

import "testing"

// CR 117.3a with CR 601.2b: a spell whose additional cost is "discard X
// cards" (Vengeful Dreams, SVar:X:Count$xPaid over Cost$ W W
// Discard<X/Card.card(s)>) is castable at X=0 -- pay the printed mana, no
// discard, no targets (TargetMin$ X resolves to 0). gorge's priority walk
// once withheld the offer entirely, so its menu was pass-only where the
// kernel offered the X=0 cast (decision 115 of 115; tail-19 pair 21).
func TestMBCVengefulDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_x_zero_cast_withheld")
}
