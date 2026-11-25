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
	// RepeatEach over the remembered cards (kop.RepeatRemembered): a token
	// minted for each remembered card's controller after a mass destroy or
	// exile that remembers. Stench of Evil's body asks (an unless-payment per
	// card inside one damage batch): the resumable remembered frame. Wave of
	// Vitriol's body asks too: an optional basic-land search per sacrificed
	// card for that card's controller, then the searchers' shuffle.
	"remembered cards": {
		"Rampage of the Clans", "Curse of the Swine", "Rowan, Scholar of Sparks", "Will, Scholar of Frost",
		"Stench of Evil", "Wave of Vitriol",
	},
	// RepeatEach over the tokens a preceding Token mint remembered
	// (DefinedCards$ RememberedCard): Reckless Crew's per-token optional
	// Equipment attach, the remembered frame's plain (unbatched) form.
	"remembered tokens": {
		"Reckless Crew",
	},
	// an ungated Repeat: the body MaxRepeat$ times (kop.OpRepeat). Trade
	// Secrets reads MaxRepeat$ from a ChooseNumber answer (Count$ChosenNumber).
	"fixed Repeat": {
		"Another Round", "Exponential Growth", "Full Flowering",
		"Torment of Hailfire", "Oath of Lim-Dûl", "Varchild's War-Riders",
		"Trade Secrets",
	},
	// RepeatEach over players whose body is a ChooseCard naming the repeated
	// player's cards: the repeated player (or the resolving controller) picks
	// the sacrifice/exile/tax target, and the post-loop node reads the picks.
	"per-player ChooseCard": {
		"Soul Shatter", "Flare of Malice", "Rite of Ruin",
		"Grave Consequences", "Watchers of the Dead",
		"Disciple of Caelus Nin", "Fortunate Few",
		"Ultimate Magic: Meteor", "Thieves' Auction",
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
