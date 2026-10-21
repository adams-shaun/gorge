package effects

import "testing"

// TestSplitDefenderSpec pins the attacked-defender split: player grammar on
// one side, permanent specs on the other, with the filter grammar's comma
// split (a comma inside named<...> is part of the name).
func TestSplitDefenderSpec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		spec, players, permanents string
	}{
		{"You", "You", ""},
		{"Player.Opponent+isMonarch", "Player.Opponent+isMonarch", ""},
		{"Battle", "", "Battle"},
		{"You,Planeswalker.YouCtrl", "You", "Planeswalker.YouCtrl"},
		{"You,Card.EffectSource", "You", "Card.EffectSource"},
		{"Opponent,Planeswalker.OppCtrl", "Opponent", "Planeswalker.OppCtrl"},
		{"Card.namedKorlash, Heir to Blackblade,You", "You", "Card.namedKorlash, Heir to Blackblade"},
		{"!Planeswalker.YouCtrl,Player.withMostLife", "Player.withMostLife", "!Planeswalker.YouCtrl"},
		{"", "", ""},
	}
	for _, c := range cases {
		players, permanents := SplitDefenderSpec(c.spec)
		if players != c.players || permanents != c.permanents {
			t.Errorf("SplitDefenderSpec(%q) = (%q, %q), want (%q, %q)",
				c.spec, players, permanents, c.players, c.permanents)
		}
	}
}
