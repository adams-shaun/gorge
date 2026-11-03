package cards_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestNormalizeNameFoldsDiacritics(t *testing.T) {
	for _, c := range []struct{ marked, plain string }{
		{"Dáin Ironfoot", "Dain Ironfoot"},
		{"Lim-Dûl's Vault", "Lim-Dul's Vault"},
		{"Mjölnir, Hammer of Thor", "Mjolnir, Hammer of Thor"},
		{"Araña, Heart of the Spider", "Arana, Heart of the Spider"},
		{"Bespoke Bō", "Bespoke Bo"},
		{"Bartolomé del Presidio", "Bartolome del Presidio"},
		{"Éowyn, Lady of Rohan", "eowyn lady of rohan"},
		{"Robo-Piñata", "Robo-Pinata"},
	} {
		if got, want := cards.NormalizeName(c.marked), cards.NormalizeName(c.plain); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", c.marked, got, want)
		}
	}
}

// TestUnmarkedNamesResolveInTheCorpus looks up the spellings an XMage manifest
// uses for cards whose script name carries a diacritic, and checks that
// folding merges no two distinct corpus names.
func TestUnmarkedNamesResolveInTheCorpus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct{ plain, printed string }{
		{"Dain Ironfoot", "Dáin Ironfoot"},
		{"Dain's Company", "Dáin's Company"},
		{"Dain, Lord of the Iron Hills", "Dáin, Lord of the Iron Hills"},
		{"Fili the Pathfinder", "Fíli the Pathfinder"},
		{"Gloin the Mighty", "Glóin the Mighty"},
		{"Kili the Resourceful", "Kíli the Resourceful"},
		{"Oin the Brave", "Óin the Brave"},
		{"Thror's Map", "Thrór's Map"},
		{"Bartolome del Presidio", "Bartolomé del Presidio"},
		{"Mjolnir, Hammer of Thor", "Mjölnir, Hammer of Thor"},
		{"Arana, Heart of the Spider", "Araña, Heart of the Spider"},
		{"Bespoke Bo", "Bespoke Bō"},
	} {
		got, ok := reg.Lookup(c.plain)
		if !ok || got.Faces[0].Name != c.printed {
			t.Errorf("Lookup(%q) = %v, want the card printed %q", c.plain, ok, c.printed)
		}
	}
	seen := map[string]string{}
	for _, c := range reg.AllCards() {
		if c == nil || len(c.Faces) == 0 {
			continue
		}
		name := c.Faces[0].Name
		k := cards.NormalizeName(name)
		if prev, ok := seen[k]; ok && prev != name {
			t.Errorf("%q and %q fold to one key %q", prev, name, k)
		}
		seen[k] = name
	}
}
