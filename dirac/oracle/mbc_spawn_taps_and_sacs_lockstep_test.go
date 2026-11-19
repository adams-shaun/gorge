package oracle

import "testing"

// CR 602.2b, 605.1a: under Path of Annihilation each untapped, non-sick
// Eldrazi Spawn adds a granted "{T}: Add one mana of any color" beside its
// printed "Sacrifice this creature: Add {C}", so two such Spawns and two sick
// ones fund Imoti, Celebrant of Bounty's {3}{G}{U} (two taps, three
// sacrifices) with every land tapped. gorge offers the cast; the kernel's
// affordability left it out (dr-mbc-babb pair 27, decision 175).
func TestMBCImotiCelebrantOfBountyReprosLockstep(t *testing.T) {
	stdReproNamed(t, "spawn_taps_and_sacs_fund_five_mana_cast")
}
