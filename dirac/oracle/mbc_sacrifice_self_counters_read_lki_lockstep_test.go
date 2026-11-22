package oracle

import "testing"

// CR 608.2h: Magmasaur's upkeep trigger asks "you may remove a +1/+1 counter
// from CARDNAME"; the decline sacrifices Magmasaur and then deals damage
// equal to the number of +1/+1 counters on it. The sacrifice instruction has
// moved the source to the graveyard by the time the damage amount is
// determined, so the count reads the source's last-known information -- the
// counters it left the battlefield with (five) -- not the card in the
// graveyard, whose counters CR 122.2 has already made cease to exist. Gorge
// once read the live graveyard card and dealt 0 damage, sparing every
// creature dirac killed (first divergence: projection Cards[5].Zone,
// decision 689 of the recorded game). The ticket was located on Dromoka
// Dunecaster at land 296be87079; the intervening merges moved this pair's
// first divergence to Magmasaur's sacrifice, which is the same game and the
// cause this recording pins.
func TestMBCDromokaDunecasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacrifice_self_counters_read_lki")
}
