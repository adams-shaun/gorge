package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHKeywordsSupported pins the edh-keywords class's pay-any-amount
// additional-cost static: Chorus of the Conclave's bare K: sentence ("As an
// additional cost to cast creature spells, you may pay any amount of mana. If
// you do, that creature enters with that many additional +1/+1 counters on
// it.") was the only thing keeping it out of full Commander-deck support. With
// the coverage report's shape gate set (the predicate make report and
// forgec edhcover share) it must now be counted playable.
func TestEDHKeywordsSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range edhKeywordCards {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}

var edhKeywordCards = []string{
	"Chorus of the Conclave",
}
