package oracle

import "testing"

// CR 106.6 with CR 605.3a: Mishra's Workshop's {C}{C}{C} is spendable only on
// artifact spells, so the colourless mana a two-mana Prized Statue leaves of
// it floats under that restriction and pays no activated ability. Silverquill
// Campus's {4},{T}: Scry 1 then has too little other mana, and gorge offers
// no activation of it. The kernel floated the surplus as ordinary mana and
// offered the Scry (decision 488 of the tail-29 pair that filed the ticket,
// recorded here on the custom table its four cards need).
func TestMBCSilverquillCampusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "workshop_surplus_floats_restricted")
}
