package cards

import "testing"

// TestSubstituteTextWordRules pins CR 612.2's word rule: whole words in
// either case, the "non" negation kept, the landwalk and plural suffixes
// admitted, other letters after the word refused, and a card name (a word
// after "named") left alone.
func TestSubstituteTextWordRules(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ text, from, to, want string }{
		{"Creature.Elf+Other", "Elf", "Vampire", "Creature.Vampire+Other"},
		{"Protection from black", "Black", "White", "Protection from white"},
		{"Creature.nonBlack", "Black", "Red", "Creature.nonRed"},
		{"target non-black creature", "Black", "Red", "target non-red creature"},
		{"Landwalk:Swamp", "Swamp", "Island", "Landwalk:Island"},
		{"Swampwalk", "Swamp", "Island", "Islandwalk"},
		{"all Swamps", "Swamp", "Island", "all Islands"},
		{"Elfhame Palace", "Elf", "Vampire", "Elfhame Palace"},
		{"Card.namedLlanowar Elves", "Elf", "Vampire", "Card.namedLlanowar Elves"},
		{"Card.namedElvish Champion+Elf", "Elf", "Vampire", "Card.namedElvish Champion+Vampire"},
		{"Blackmail", "Black", "Red", "Blackmail"},
		{"", "Elf", "Vampire", ""},
	} {
		if got := SubstituteTextWord(c.text, c.from, c.to); got != c.want {
			t.Errorf("SubstituteTextWord(%q, %q, %q) = %q, want %q", c.text, c.from, c.to, got, c.want)
		}
	}
}
