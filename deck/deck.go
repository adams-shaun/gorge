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

// File is the on-disk shape. Name and Format are authoring metadata; only
// Cards decides what is dealt.
type File struct {
	Name   string  `json:"name"`
	Format string  `json:"format"`
	Cards  []Entry `json:"cards"`
}

// Entry is one line of a deck list.
type Entry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
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
