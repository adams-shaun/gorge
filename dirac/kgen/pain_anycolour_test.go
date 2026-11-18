package kgen

import "testing"

// TestLowerPainAnyColourMana: a painland whose painful ability makes one
// mana of any colour (Tarnished Citadel's 3 damage, Grand Coliseum's 1,
// beside the {C} ability) lowers.
func TestLowerPainAnyColourMana(t *testing.T) {
	const head = "Name:Citadel Probe\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\n"
	for _, c := range []struct {
		name string
		dmg  string
	}{{"three", "3"}, {"one", "1"}} {
		src := head + "A:AB$ Mana | Cost$ T | Produced$ Any | SubAbility$ DBPain | SpellDescription$ Add one mana of any color.\n" +
			"SVar:DBPain:DB$ DealDamage | NumDmg$ " + c.dmg + " | Defined$ You\n"
		if _, rs := kwardpowBuild(t, "Citadel Probe", src); len(rs) != 0 {
			t.Errorf("%s damage: refused: %v", c.name, rs)
		}
	}
}
