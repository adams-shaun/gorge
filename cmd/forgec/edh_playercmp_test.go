package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhPlayerCompareCards is the PlayerCount property class the
// edh-playercmp ticket covers: cards whose SVar reads a PlayerCount<Group>$
// <Property> body whose property compares another player's board or life
// with yours. Each entry is asserted fully supported by
// cards.Registry.Unsupported with effects.Supported() -- make report's
// predicate -- so an imported Commander deck holding one is never counted
// unplayable over a missing count: primitive.
var edhPlayerCompareCards = []string{
	"Oreskos Explorer",
	"Birth of the Imperium",
	"Voice of Many",
	"Stoic Farmer",
	"Priest of the Blessed Graf",
	"Surveyor's Scope",
	"Heidegger, Shinra Executive",
	"Great Unclean One",
	"Sudden Salvation",
}

// TestEDHPlayerCompareSupported pins the whole class at the coverage
// predicate: every listed card reports no missing primitives. A card a
// later ticket cannot finish must be dropped from the list NAMED in the
// ticket's report, never silently.
func TestEDHPlayerCompareSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	// The predicate is exactly make report's, gate included; none of the
	// class's cards carries an ETB Clone body today, so the gate is a no-op
	// here — but if one ever grows one, the check must say so.
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range edhPlayerCompareCards {
		card, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %q", name)
		}
		if missing := r.Unsupported(card, sup); len(missing) != 0 {
			t.Errorf("%s counted unsupported: %v", name, missing)
		}
	}
}
