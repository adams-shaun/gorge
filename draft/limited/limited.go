// Package limited holds the shared types of the draft stack: the pure
// vocabulary every draft/limited/ package (booster, draft, build, pickbot) speaks.
// It carries no behaviour and imports nothing but the standard library, so it
// can be embedded in event logs and wire payloads alike.
package limited

import "fmt"

// Rarity is a card's printing rarity in a limited context. The byte values
// are the single letters the set data and logs spell out.
type Rarity byte

// The rarity letters. L is the land slot (basic or gain land); S is a
// special printing outside the main runs.
const (
	Common   Rarity = 'C'
	Uncommon Rarity = 'U'
	Rare     Rarity = 'R'
	Mythic   Rarity = 'M'
	Land     Rarity = 'L'
	Special  Rarity = 'S'
)

// String returns the rarity's single letter.
func (r Rarity) String() string { return string(rune(r)) }

// MarshalJSON encodes the rarity as its single letter, matching the set
// data's JSON shape.
func (r Rarity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + r.String() + `"`), nil
}

// UnmarshalJSON reads a single-letter rarity.
func (r *Rarity) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' || len(b) != 3 {
		return fmt.Errorf("limited: rarity must be one quoted letter, got %s", b)
	}
	*r = Rarity(b[1])
	return nil
}

// CardRef is one card instance in an event: the minimal, serialisable facts
// every stage of the draft stack passes around. Colour and mana value come
// from the cards.Registry by Name when needed; a CardRef stays small.
type CardRef struct {
	// ID is unique within an event, assigned in pack-open order ("c117").
	ID string `json:"id"`
	// Name is the card's name as the corpus spells it.
	Name string `json:"name"`
	// Set is the printing set code, for art lookup later.
	Set string `json:"set"`
	// Number is the collector number of the printing.
	Number string `json:"number"`
	// Rarity is the printing's rarity.
	Rarity Rarity `json:"rarity"`
}
