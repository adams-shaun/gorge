package effects

import "testing"

// A Permanent base on any comma-separated alternative of a Dig window's
// ChangeValid$ reads as a permanent card, not as an object on the battlefield
// (CR 110.1, 108.1): There's Coffee in that Nebula's second alternative.
func TestPermanentCardSpecRewritesEveryAlternative(t *testing.T) {
	for in, want := range map[string]string{
		"Plains,Permanent.nonLand+cmcLE3": "Plains,PermanentCard.nonLand+cmcLE3",
		"Land,Permanent.Legendary+cmcLEX": "Land,PermanentCard.Legendary+cmcLEX",
		"Creature,Card.Permanent+nonLand": "Creature,PermanentCard.nonLand",
		"Permanent.Land,Permanent":        "PermanentCard.Land,PermanentCard",
		"Plains,Island":                   "Plains,Island",
		"Permanent.nonLand+cmcLE3,Plains": "PermanentCard.nonLand+cmcLE3,Plains",
	} {
		if got := permanentCardSpec(in); got != want {
			t.Errorf("permanentCardSpec(%q) = %q, want %q", in, got, want)
		}
	}
}
