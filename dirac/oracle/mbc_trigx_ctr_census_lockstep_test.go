package oracle

import (
	"testing"
)

// A cycled Decree of Justice's "you may pay {X}" announces X up to the most
// mana its payer can produce (CR 107.3i, 601.2b): the floating pool plus every
// mana ability usable now. Beside an untapped Forest and a Wall of Roots whose
// counter-cost ability ("Put a -0/-1 counter on Wall of Roots: Add {G}")
// taps nothing, the kernel's capped census (availForCapped, the trigger X
// announcement's one-tap cap) read only tap- and sacrifice-cost sources and
// dropped the Wall, offering X to 1 where gorge offers 2. The recorded game's
// first divergence is the kernel's missing X = 2 option at decision 290
// (options options, dr-mbc-bbaa pair 72).
func TestMBCAssaultFormationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigx_ctr_census")
}
