package neutral_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

func TestCatalogDecksResolveToGorgeCatalog(t *testing.T) {
	reg := testcorpus.Registry(t)
	for _, cat := range catalog.Decks() {
		d, err := neutral.ResolveDeck(reg, cat.Rows)
		if err != nil {
			t.Fatalf("%s: %v", cat.CatalogID, err)
		}
		if len(d.Cards) != 60 {
			t.Errorf("%s resolves to %d cards", cat.CatalogID, len(d.Cards))
		}
		if got, want := wire.DeckID(d.Rows), cat.DeckID(); got != want {
			t.Errorf("%s renamed deck_id %s, gorge catalog %s", cat.CatalogID, got, want)
		}
	}
}

func TestResolveNamesFacesAndRefusesNearMisses(t *testing.T) {
	reg := testcorpus.Registry(t)
	for _, c := range []struct {
		name, oracle string
		face         int
	}{
		{"Lightning Bolt", "Lightning Bolt", 0},
		{"Troll of Khazad-dum", "Troll of Khazad-dûm", 0},
		{"Troll of Khazad-dûm", "Troll of Khazad-dûm", 0},
		{"Sagu Wildling", "Sagu Wildling // Roost Seek", 0},
		{"Roost Seek", "Sagu Wildling // Roost Seek", 1},
		{"Sagu Wildling // Roost Seek", "Sagu Wildling // Roost Seek", 0},
		{"Vector Glider", "The Modern Age // Vector Glider", 1},
	} {
		ref, err := neutral.Resolve(reg, c.name)
		if err != nil || ref.Oracle != c.oracle || ref.Face != c.face {
			t.Errorf("%q resolves to %q face %d (%v), want %q face %d", c.name, ref.Oracle, ref.Face, err, c.oracle, c.face)
		}
	}
	for _, name := range []string{"lightning bolt", "Lightning Bolt!", "troll of khazad-dum", "Lorien Revealed ", "Roost Seek // Sagu Wildling", "Not A Card"} {
		if ref, err := neutral.Resolve(reg, name); err == nil {
			t.Errorf("%q resolved to %q", name, ref.Oracle)
		}
	}
}

func TestCoverageReportsUnsupportedCards(t *testing.T) {
	reg := testcorpus.Registry(t)
	for _, name := range []string{"Incinerate"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the compiled corpus registry", name)
		}
		gaps := neutral.Coverage(reg, neutral.Deck{Cards: []*cards.Card{c}})
		if len(gaps) != 1 || gaps[0].Name != name || len(gaps[0].Missing) == 0 {
			t.Errorf("Coverage(%s) = %+v, want one gap naming its missing primitives", name, gaps)
		}
	}
	bolt, ok := reg.Lookup("Lightning Bolt")
	if !ok {
		t.Fatal("Lightning Bolt not in the compiled corpus registry")
	}
	if gaps := neutral.Coverage(reg, neutral.Deck{Cards: []*cards.Card{bolt}}); len(gaps) != 0 {
		t.Errorf("Coverage(Lightning Bolt) = %+v, want none", gaps)
	}
}

func TestResolveDeckRefusesRowsNamingOneCard(t *testing.T) {
	reg := testcorpus.Registry(t)
	rows := []wire.DeckRow{{Name: "Troll of Khazad-dum", Count: 1}, {Name: "Troll of Khazad-dûm", Count: 1}}
	if _, err := neutral.ResolveDeck(reg, rows); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("duplicate rows resolved: %v", err)
	}
	if _, err := neutral.ResolveDeck(reg, []wire.DeckRow{{Name: "Mountain", Count: 0}}); err == nil {
		t.Fatal("zero-count row resolved")
	}
}
