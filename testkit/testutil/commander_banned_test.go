package testutil

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/deck"
)

// TestCommanderBannedNamesResolveInCorpus catches spelling drift between the
// banned list and the corpus's card names.
func TestCommanderBannedNamesResolveInCorpus(t *testing.T) {
	if len(deck.CommanderBannedNames) == 0 {
		t.Fatal("precondition: empty banned list")
	}
	reg := CorpusRegistry(t)
	for _, n := range deck.CommanderBannedNames {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Errorf("%q does not resolve in the corpus", n)
			continue
		}
		if !deck.IsCommanderBanned(c) {
			t.Errorf("IsCommanderBanned(%q) = false", n)
		}
	}
}
