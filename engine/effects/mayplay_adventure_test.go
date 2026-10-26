package effects

import "testing"

func TestMayPlayAdventureSpec(t *testing.T) {
	for _, c := range []struct {
		in, want string
		adv      bool
	}{
		{"Card.IsRemembered+Adventure", "Card.IsRemembered+AdventureCard", true},
		{"Card.Adventure", "Card.AdventureCard", true},
		{"Instant.Adventure,Sorcery.Adventure", "Instant.AdventureCard,Sorcery.AdventureCard", true},
		{"Card.Adventure,Dragon", "Card.AdventureCard,Dragon", false},
		{"Card.IsRemembered", "Card.IsRemembered", false},
		{"Card.IsRemembered+AdventureCard", "Card.IsRemembered+AdventureCard", false},
		{"Card.nonAdventure", "Card.nonAdventure", false},
	} {
		got, adv := MayPlayAdventureSpec(c.in)
		if got != c.want || adv != c.adv {
			t.Errorf("MayPlayAdventureSpec(%q) = %q,%v want %q,%v", c.in, got, adv, c.want, c.adv)
		}
	}
}
