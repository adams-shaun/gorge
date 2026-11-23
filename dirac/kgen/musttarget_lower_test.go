package kgen

import (
	"path/filepath"
	"testing"
)

// TestMustTargetLowers checks the Flagbearer rule's static lowering: the
// corpus's three carriers lower cleanly with one MustTarget static whose
// ValidTarget$ filter is the Flagbearer subtype, and the ValidZone$ is the
// battlefield.
func TestMustTargetLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join("..", "cardset", "pools", "fixtures", "musttarget", "cards.txt")
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}
	for _, name := range []string{"Coalition Flag", "Coalition Honor Guard", "Standard Bearer"} {
		c := byName[name]
		if c == nil || len(c.Refusals) != 0 {
			t.Fatalf("%s: %+v, want lowered clean", name, c)
		}
		var found []lStatic
		for _, st := range c.Statics {
			if st.Kind == "MustTarget" {
				found = append(found, st)
			}
		}
		if len(found) != 1 {
			t.Fatalf("%s: %d MustTarget statics, want 1", name, len(found))
		}
		st := found[0]
		if st.F == nil || len(st.F.Clauses) != 1 || st.F.Clauses[0].Subtype != "Flagbearer" {
			t.Errorf("%s: filter %v, want the Flagbearer subtype", name, st.F)
		}
	}
}
