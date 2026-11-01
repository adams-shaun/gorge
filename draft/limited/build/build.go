// Package build turns a drafted pool into an engine-loadable deck. It is pure — a deck is validated
// against the pool it came from, lands are split by coloured-pip share and
// the result is written out as a deck.File. It imports only the limited
// vocabulary and the deck-list format.
package build

import (
	"fmt"
	"math"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/engine/deck"
)

// BasicNames names the five basics in the fixed W U B R G order every
// Basics array uses.
var BasicNames = [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// MaxBasicsPerType is the most copies of one basic land type a built deck may
// carry. MTR 7.1 imposes no maximum — basics are free and unlimited — so this
// is a hosting bound: it caps an untrusted build request before the count can
// reach the engine or the deck file.
const MaxBasicsPerType = 40

// MaxDeckSize is the most cards a built deck may contain, maindeck plus
// basics. MTR 7.1 imposes no maximum; this is a hosting bound so a crafted
// request cannot build a deck large enough to DoS an event.
const MaxDeckSize = 100

// Deck is one seat's built deck: the CardRef IDs it maindecks from its own
// pool plus the basic lands it adds free of charge (MTR 7.1 — basics are
// unlimited, and there is no other copy limit in limited play).
type Deck struct {
	// Main is the deck's non-basic cards, as IDs of the pool's CardRefs.
	// omitempty keeps a deck-less line from carrying an empty block;
	// encoding/json matches the old capitalised field names case-blind,
	// so a log written before the tags still decodes.
	Main []string `json:"main,omitempty"`
	// Basics counts each basic land by colour: W, U, B, R, G. omitempty
	// is a no-op on an array; the all-zero Basics block stays visible,
	// exactly as before the tags.
	Basics [5]int `json:"basics"`
}

// Total is the deck's card count: maindeck cards plus basics. It saturates at
// the int limits rather than overflowing, so it is safe to call on an
// untrusted Deck before Validate has bounded its counts.
func (d Deck) Total() int {
	n := len(d.Main)
	for _, c := range d.Basics {
		switch {
		case c > 0 && n > math.MaxInt-c:
			return math.MaxInt
		case c < 0 && n < math.MinInt-c:
			return math.MinInt
		}
		n += c
	}
	return n
}

// Validate checks a deck against the pool it was built from: at least 40
// cards, at most MaxDeckSize, every basic count between 0 and
// MaxBasicsPerType, every maindeck ID in the pool, no duplicate IDs. There is
// no copy limit on non-basics (MTR 7.1); basics are free and unlimited under
// the rules, so the per-type and total caps are hosting bounds.
func Validate(pool []limited.CardRef, d Deck) error {
	for i, n := range d.Basics {
		if n < 0 {
			return fmt.Errorf("build: %s count %d is negative", BasicNames[i], n)
		}
		if n > MaxBasicsPerType {
			return fmt.Errorf("build: %s count %d exceeds the %d-per-type limit", BasicNames[i], n, MaxBasicsPerType)
		}
	}
	inPool := make(map[string]struct{}, len(pool))
	for _, c := range pool {
		inPool[c.ID] = struct{}{}
	}
	switch total := d.Total(); {
	case total < 40:
		return fmt.Errorf("build: deck has %d cards, want at least 40", total)
	case total > MaxDeckSize:
		return fmt.Errorf("build: deck has %d cards, want at most %d", total, MaxDeckSize)
	}
	seen := make(map[string]struct{}, len(d.Main))
	for _, id := range d.Main {
		if _, ok := inPool[id]; !ok {
			return fmt.Errorf("build: card %q is not in the pool", id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("build: card %q is maindecked twice", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// AutoLands fills a maindeck up to 40 cards with basics, split by the
// coloured-pip share of the non-land main cards using largest remainder.
// colourOf reports, for one maindeck card ID, how many pips of each colour
// (W U B R G) its mana costs carry; a nil colourOf counts no pips. A deck
// with no coloured pips — all colourless spells, or no land slots to fill —
// splits its share over nothing and gets zero basics back.
func AutoLands(main []string, colourOf func(id string) [5]int) [5]int {
	var out [5]int
	lands := 40 - len(main)
	if lands <= 0 {
		return out
	}
	if colourOf == nil {
		colourOf = func(string) [5]int { return [5]int{} }
	}
	var pips [5]int
	total := 0
	for _, id := range main {
		for c, p := range colourOf(id) {
			pips[c] += p
			total += p
		}
	}
	if total == 0 {
		return out
	}
	// exact[i] = lands * pips[i] / total, kept in integer parts: floor plus
	// the remainder (lands*pips[i] mod total) largest remainders are handed
	// the leftover lands, ties to the lower colour index.
	var rem [5]int
	left := lands
	for i := range out {
		exact := lands * pips[i]
		out[i] = exact / total
		rem[i] = exact % total
		left -= out[i]
	}
	idx := []int{0, 1, 2, 3, 4}
	sort.SliceStable(idx, func(a, b int) bool {
		if rem[idx[a]] != rem[idx[b]] {
			return rem[idx[a]] > rem[idx[b]]
		}
		return idx[a] < idx[b]
	})
	for _, i := range idx {
		if left == 0 {
			break
		}
		out[i]++
		left--
	}
	return out
}

// ToDeckFile writes the deck out as an engine-loadable deck file, with the
// basics as ordinary entries. Card names come from the pool by ID; an ID
// the pool does not carry yields an empty name, so run Validate first.
// The file's Format and Archetype stay empty: they are authoring metadata
// the caller may fill in.
func ToDeckFile(pool []limited.CardRef, d Deck, name string) deck.File {
	nameOf := make(map[string]string, len(pool))
	for _, c := range pool {
		nameOf[c.ID] = c.Name
	}
	f := deck.File{Name: name}
	for _, id := range d.Main {
		f.Cards = append(f.Cards, deck.Entry{Name: nameOf[id], Count: 1})
	}
	for i, n := range d.Basics {
		if n > 0 {
			f.Cards = append(f.Cards, deck.Entry{Name: BasicNames[i], Count: n})
		}
	}
	return f
}
