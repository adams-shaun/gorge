// Package deck reads the repo's deck-list JSON — a bare {name, count} card
// list — and resolves it against a cards.Registry into the flat, repeated
// []*cards.Card that rules.Config.Decks wants. It is the one parser both
// the test fixtures (internal/testutil) and the match host use, so the two
// can never disagree about what a deck file means. A deck list carries no
// card text and is not the licensing hazard cards/boundary_test.go guards.
package deck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// File is the on-disk shape. Name and Format are authoring metadata,
// Commander marks a deck as a Commander deck (see ValidateCommander), and
// only Cards decides what is dealt. Commander is optional and additive: a
// deck file written before it existed parses identically (Commander is just
// empty, which means "constructed" — nothing changes for the repo's existing
// deck files), so adding it never makes an old list invalid.
type File struct {
	Name      string  `json:"name"`
	Format    string  `json:"format"`
	Archetype string  `json:"archetype"`
	Commander string  `json:"commander"`
	Cards     []Entry `json:"cards"`
}

// Entry is one line of a deck list.
type Entry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// commanderOraclePhrase is the only way the pinned corpus marks a card that
// may be a commander despite not being a legendary creature: it appears as
// prose in the Oracle text, never as a structured field or type. This is
// checked in the deck validator (not here) so cards itself stays free of
// Commander rules; the substring is the literal printed phrase on those
// scripts.
const commanderOraclePhrase = "can be your commander"

// CommanderIndex returns the flat index of f.Commander's card in the deck
// Resolve produces: the position its single copy (CR 903.4 singleton) lands
// at when entries are expanded by count in file order. This is the index
// rules.Config.Commanders expects — genesis moves that object to the
// command zone instead of the library. Computed rather than assumed 0 so a
// deck that does not print its commander first keeps working; the result is
// only meaningful for a deck that passed ValidateCommander (which
// guarantees the commander is one of the deck's entries), and a deck with
// no commander at all returns the count-expanded length, which is why
// callers gate on Commander != "" before calling it.
func (f File) CommanderIndex() int {
	cmdr := cards.NormalizeName(f.Commander)
	idx := 0
	for _, e := range f.Cards {
		if cards.NormalizeName(e.Name) == cmdr {
			return idx
		}
		idx += e.Count
	}
	return idx
}

// IsCommanderEligible reports whether a card may be a commander: it is a
// legendary creature, or one of the cards the corpus marks as saying it can
// be a commander (planeswalker-legends, Partner/choose-a-background cases,
// which the corpus carries only as Oracle prose — see the report).
func IsCommanderEligible(c *cards.Card) bool {
	for _, f := range c.Faces {
		if f.IsLegendary() && f.IsCreature() {
			return true
		}
	}
	for _, f := range c.Faces {
		if strings.Contains(f.Oracle, commanderOraclePhrase) {
			return true
		}
	}
	return false
}

// Parse decodes a deck file and rejects the shapes that would otherwise
// fail later in a less obvious place: no cards, an unnamed entry, a
// non-positive count.
func Parse(raw []byte) (File, error) {
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, fmt.Errorf("deck: %w", err)
	}
	if len(f.Cards) == 0 {
		return File{}, fmt.Errorf("deck: no cards")
	}
	for i, e := range f.Cards {
		if e.Name == "" {
			return File{}, fmt.Errorf("deck: entry %d has no name", i)
		}
		if e.Count <= 0 {
			return File{}, fmt.Errorf("deck: %q has count %d", e.Name, e.Count)
		}
	}
	return f, nil
}

// Resolve looks every entry up in r (which normalises names itself) and
// expands it by its count, in file order. The first unknown card is named
// in the error.
func (f File) Resolve(r *cards.Registry) ([]*cards.Card, error) {
	var out []*cards.Card
	for _, e := range f.Cards {
		c, ok := r.Lookup(e.Name)
		if !ok {
			return nil, fmt.Errorf("deck %q: card %q is not in the registry", f.Name, e.Name)
		}
		for i := 0; i < e.Count; i++ {
			out = append(out, c)
		}
	}
	return out, nil
}

// Load reads, parses and resolves one deck file.
func Load(r *cards.Registry, path string) (File, []*cards.Card, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, nil, fmt.Errorf("deck: %w", err)
	}
	f, err := Parse(raw)
	if err != nil {
		return File{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	cs, err := f.Resolve(r)
	if err != nil {
		return File{}, nil, err
	}
	return f, cs, nil
}

// Stem is the deck's short name: the file name without directory or
// extension ("decks/mono-red-goblins.json" -> "mono-red-goblins"). The host
// names seats after it.
func Stem(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// ValidateCommander enforces the Commander deck-construction rules (CR 903.4:
// exactly 100 cards including the commander and, except basic lands, no two
// cards with the same English name; CR 903.5: every card's colour identity is
// a subset of the commander's) for a deck file that names a commander. It is
// called explicitly — never from Parse — and a constructed deck (a File with
// no commander designation) is not subject to any of it and returns nil, so
// wiring the validator in never changes how an existing deck file is read.
// Every violation is reported together and each names the offending card (or
// the commander).
func (f File) ValidateCommander(r *cards.Registry) error {
	if f.Commander == "" {
		return nil // constructed: no Commander rules apply
	}

	// Resolve the commander first: the whole check hangs off its identity and
	// eligibility, so a missing or illegal commander is the first thing named.
	cmdrKey := cards.NormalizeName(f.Commander)
	cmdr, ok := r.Lookup(f.Commander)
	if !ok {
		return fmt.Errorf("commander %q is not in the registry", f.Commander)
	}
	if !IsCommanderEligible(cmdr) {
		return fmt.Errorf("commander %q is not a legendary creature or a card that says it can be your commander", f.Commander)
	}
	cmdrID := cmdr.ColourIdentity()

	var errs []string

	// Resolve every entry once and, keyed by normalised name, remember it so
	// the singleton and colour checks run once per distinct card.
	byName := map[string]*cards.Card{}
	for _, e := range f.Cards {
		k := cards.NormalizeName(e.Name)
		c, ok := r.Lookup(e.Name)
		if !ok {
			return fmt.Errorf("deck %q: card %q is not in the registry", f.Name, e.Name)
		}
		byName[k] = c
	}

	// CR 903.4 — exactly 100 cards including the commander.
	total := 0
	for _, e := range f.Cards {
		total += e.Count
	}
	if total != 100 {
		errs = append(errs, fmt.Sprintf("%s: %d cards, but a Commander deck must have exactly 100 including the commander", f.Name, total))
	}

	// CR 903.4 — the commander must actually be one of the deck's 100 cards.
	if _, isEntry := byName[cmdrKey]; !isEntry {
		errs = append(errs, fmt.Sprintf("commander %q is not in the deck's card list", f.Commander))
	}

	// CR 903.4 singleton (basic lands excepted) and CR 903.5 colour identity.
	for _, e := range f.Cards {
		c := byName[cards.NormalizeName(e.Name)]
		basic := isBasicLand(c)
		if e.Count > 1 && !basic {
			errs = append(errs, fmt.Sprintf("card %q appears %d times; a Commander deck is singleton except basic lands", e.Name, e.Count))
		}
		if id := c.ColourIdentity(); id&^cmdrID != 0 {
			errs = append(errs, fmt.Sprintf("card %q has colour identity {%s}, outside commander %q's {%s}",
				e.Name, colourNames(id), f.Commander, colourNames(cmdrID)))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("commander deck invalid:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// isBasicLand reports whether a card is a basic land — the singleton rule's
// sole exemption (CR 903.4). A card is basic if any face carries the Basic
// land type.
func isBasicLand(c *cards.Card) bool {
	for _, f := range c.Faces {
		if f.IsBasic() && f.IsLand() {
			return true
		}
	}
	return false
}

// colourNames renders a colour-identity bitmask as the human set, e.g.
// "white/blue" or "colourless" for the empty set, for violation messages.
func colourNames(m uint8) string {
	order := []struct {
		bit  uint8
		name string
	}{
		{cards.ColourWhite, "white"},
		{cards.ColourBlue, "blue"},
		{cards.ColourBlack, "black"},
		{cards.ColourRed, "red"},
		{cards.ColourGreen, "green"},
	}
	var parts []string
	for _, o := range order {
		if m&o.bit != 0 {
			parts = append(parts, o.name)
		}
	}
	if len(parts) == 0 {
		return "colourless"
	}
	return strings.Join(parts, "/")
}
