package deck

import "github.com/adams-shaun/gorge/engine/cards"

// substituteBasics are the basic lands SubstituteBasic counts, and the colour
// each of the first five stands for (Wastes stands for none).
var substituteBasics = [...]struct {
	name   string
	colour uint8
}{
	{"Plains", cards.ColourWhite},
	{"Island", cards.ColourBlue},
	{"Swamp", cards.ColourBlack},
	{"Mountain", cards.ColourRed},
	{"Forest", cards.ColourGreen},
	{"Wastes", 0},
}

// SubstituteBasic is the basic land that stands in for a card f cannot play:
// the deck's most common basic land (the first in deck order on a tie). A deck
// with no basic land gets the first of Plains, Island, Swamp, Mountain, Forest
// whose colour is in its commanders' colour identity, and Wastes when that
// identity is empty or a commander does not resolve in r.
func SubstituteBasic(f File, r *cards.Registry) string {
	best, n := "", 0
	for _, e := range f.Cards {
		if isSubstituteBasic(e.Name) && e.Count > n {
			best, n = e.Name, e.Count
		}
	}
	if best != "" {
		return best
	}
	var id uint8
	for _, name := range f.CommanderNames() {
		c, ok := r.Lookup(name)
		if !ok {
			return "Wastes"
		}
		id |= c.ColourIdentity()
	}
	for _, b := range substituteBasics {
		if b.colour&id != 0 {
			return b.name
		}
	}
	return "Wastes"
}

func isSubstituteBasic(name string) bool {
	for _, b := range substituteBasics {
		if b.name == name {
			return true
		}
	}
	return false
}
