package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhKeywordCards is the EDH keyword class: imported Commander decks keep
// holding these, and each was blocked from full gorge support only by one
// missing primitive key (kw:Epic for Enduring Ideal and Eternal Dominion).
//
// Chorus of the Conclave's bare K: sentence ("As an additional cost to cast
// creature spells, you may pay any amount of mana. If you do, that creature
// enters with that many additional +1/+1 counters on it.") was the
// pay-any-amount additional-cost static.
var edhKeywordCards = []string{
	"Chorus of the Conclave",
	"Enduring Ideal",
	"Eternal Dominion",
}

// TestEDHKeywordsSupported asserts every card in the EDH keyword class is
// primitive-complete and counted playable by the coverage report's gate —
// the same predicate `make report` and `forgec edhcover` use.
func TestEDHKeywordsSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range edhKeywordCards {
		card, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %q", name)
			continue
		}
		if m := r.Unsupported(card, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
