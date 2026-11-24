package kgen

import "testing"

// repeatEachSliceCards are the cards the gap-repeateach slice lowers, by form.
var repeatEachSliceCards = map[string][]string{
	// RepeatEach over players, a lone DealDamage to the repeated player under
	// DamageMap$ True (the loop is one damage batch: kop.RepeatEachDmgBatch)
	"damage loop": {
		"Acidic Soil", "Heartless Hidetsugu", "Mjölnir, Storm Hammer", "Stormbreath Dragon",
		"Sunspine Lynx", "Treacherous Terrain", "Typhoon",
	},
	// RepeatEach over the opponents minting a token tapped and attacking with
	// the plain TokenAttacking$ Remembered / RememberedPlayer (no planeswalker
	// ask: kop.TokAttackPlayer). Mardu Siegebreaker's copy of an exiled card
	// stays refused.
	"token attacking the repeated player": {
		"Ainok Strike Leader", "Endless Foot Assault", "Stampede Surfer", "Veteran Soldier",
	},
	// an ungated Repeat: the body MaxRepeat$ times (kop.OpRepeat). Trade
	// Secrets reads MaxRepeat$ from a ChooseNumber answer (Count$ChosenNumber).
	"fixed Repeat": {
		"Another Round", "Exponential Growth", "Full Flowering",
		"Torment of Hailfire", "Oath of Lim-Dûl", "Varchild's War-Riders",
		"Trade Secrets",
	},
}

func TestRepeatEachSlice(t *testing.T) {
	for form, names := range repeatEachSliceCards {
		for _, name := range names {
			if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
				t.Errorf("%s (%s): refusals = %v, want none", name, form, c.Refusals)
			}
		}
	}
}
