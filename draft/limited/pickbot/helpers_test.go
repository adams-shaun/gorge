package pickbot_test

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// corpus loads the shared card corpus once for the package's tests. A
// checkout with no .cards/ makes this skip — which reads green and tests
// nothing — so every corpus-backed assertion below lives behind it.
func corpus(t *testing.T) *cards.Registry {
	t.Helper()
	return testutil.CorpusRegistry(t)
}

// lookupOf wraps a registry into the pickbot Lookup shape.
func lookupOf(reg *cards.Registry) pb.CardLookup {
	return func(name string) *cards.Card {
		c, _ := reg.Lookup(name)
		return c
	}
}

// lookupCard resolves one name in the corpus or fails the test.
func lookupCard(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok || c == nil || len(c.Faces) == 0 {
		t.Fatalf("corpus has no card %q", name)
	}
	return c
}

// ref builds a CardRef with an id, name and rarity.
func ref(id, name string, rarity limited.Rarity) limited.CardRef {
	return limited.CardRef{ID: id, Name: name, Set: "FDN", Rarity: rarity}
}

// sheetRarities maps each sheet card's name to its printing rarity from the
// embedded booster spec. The rarity comes off the SheetCard's own field (the
// manifest's uppercase letter), never off the sheet key: the keys are the
// lowercase sheet names ("common", "uncommon", ...) while limited.Rarity
// constants are uppercase ('C', 'U', ...), so reading the key's first byte
// would hand the heuristic a rarity it scores as 0.
func sheetRarities(t *testing.T, spec setdata.Spec) map[string]limited.Rarity {
	t.Helper()
	rarity := map[string]limited.Rarity{}
	for _, sheet := range spec.Config.Sheets {
		for _, c := range sheet.Cards {
			rarity[c.Name] = c.Rarity
		}
	}
	if len(rarity) == 0 {
		t.Fatal("booster spec yielded no sheet cards")
	}
	return rarity
}

// fdnMainSetNames lists the DISTINCT names across every FDN sheet card of
// the embedded booster spec (the default collation's sheets, special-guest
// and wildcard included), sorted. Entries repeat across sheets —
// foilReplacement mirrors the main-set printings, wildcard repeats
// uncommons — so the count that means anything is the distinct-name count.
func fdnMainSetNames(t *testing.T) []string {
	t.Helper()
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	seen := map[string]bool{}
	for sheetName, sheet := range spec.Config.Sheets {
		if len(sheet.Cards) == 0 {
			t.Fatalf("FDN sheet %q is empty", sheetName)
		}
		for _, c := range sheet.Cards {
			seen[c.Name] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("booster spec yielded no sheet cards")
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
