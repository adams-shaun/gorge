package oracle

import "testing"

// Incubation Druid's "{T}: Add one mana of any type that a land you control
// could produce. If Incubation Druid has a +1/+1 counter on it, add three
// mana of that type instead" carries its amount in an Amount$ SVar, so the
// shared payment census prices no production for it (an indeterminate amount
// yields no guaranteed one). The cast window's probe is the layer that prices
// dynamic amounts, but it read the ability's printed body -- blank for a
// ManaReflected, hence the executor's one-colourless default -- so it priced
// three {C}: mana that cannot pay a {G} pip (CR 107.5b). Casting the
// reflection's own colours ({G} or {B} off the board's Forests and Swamps,
// CR 605.1a) made Ayula's Influence {G}{G}{G} affordable, and gorge dropped
// it from the priority menu while the kernel kept it. Recorded as tail-55
// pair 3 (surface full, mulligans 0, seed 595286).
func TestMBCAyulaSInfluenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_svar_amount_colourless_default")
}
