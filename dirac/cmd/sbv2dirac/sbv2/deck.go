package sbv2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// ReadDeck reads a gorge deck file as a catalog deck named by its file stem.
func ReadDeck(path string) (catalog.Deck, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return catalog.Deck{}, err
	}
	f, err := deck.Parse(raw)
	if err != nil {
		return catalog.Deck{}, fmt.Errorf("%s: %w", path, err)
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".json")
	d := catalog.Deck{CatalogID: stem, Name: stem}
	for _, e := range f.Cards {
		d.Rows = append(d.Rows, wire.DeckRow{Name: e.Name, Count: e.Count})
	}
	return d, nil
}
