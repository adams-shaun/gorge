package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhKeywordCards is the EDH keyword class this ticket covers: imported
// Commander decks keep holding these, and each is blocked from full gorge
// support only by one missing primitive key.
//
// Enduring Ideal, Eternal Dominion (kw:Epic) and Chorus of the Conclave (the
// pay-any-amount additional-cost static) are deliberately absent: their
// mechanisms were split into follow-up tickets (r2 report, "Split-off work") —
// Epic needs copy-this-spell-at-upkeep machinery with no engine precedent and
// Chorus a new pay-any-amount additional-cost class — and an unfinished
// mechanism is never registered honestly.
var edhKeywordCards = []string{
	"Nazgûl",
	"Seven Dwarves",
	"Master of Cruelties",
	"Blast-Furnace Hellkite",
	"Patron of the Nezumi",
	"Nacatl War-Pride",
	"Garza's Assassin",
	"Icefall",
	"Marhault Elsdragon",
	"Fleshwrither",
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
