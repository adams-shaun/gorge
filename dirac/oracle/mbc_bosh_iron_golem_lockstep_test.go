package oracle

import "testing"

// Bosh, Iron Golem's "{3}{R}, Sacrifice an artifact" was offered by the kernel
// but not by gorge at p1's priority decision 966 (tail-55 pair 295, seed
// 839419): every land tapped, Cascade Bluffs just played beside an Incubation
// Druid holding counters. gorge's PotentialMana bound prices the Druid's
// reflected Amount$ as colourless alone and cannot cover Bluffs' {U/R} filter
// fee with it, so no red reaches the bound and autoPayOffer refuses the
// activation before any search (CR 605.3a, 601.2g); the kernel's gaps walk had
// funded the fee from the Druid's {C}. scriptOffers now drops a play the bound
// cannot pay beside a filter and a live reflected Amount$ source.
func TestMBCBoshIronGolemReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bosh_iron_golem_activate")
}
