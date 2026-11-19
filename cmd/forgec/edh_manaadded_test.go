package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHManaAddedSupported pins the coverage report for the trig:ManaAdded
// class (edh ticket edh-manaadded): Caged Sun's two `T:Mode$ ManaAdded`
// triggered-mana-ability lines (CR 605.1b) must be implemented AND
// registered, so the card counts playable for imported Commander decks. A
// card that cannot be finished stays out of the list and is named in the
// ticket's report, never silently dropped.
func TestEDHManaAddedSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	sun, ok := r.Lookup("Caged Sun")
	if !ok {
		t.Fatal("corpus missing Caged Sun")
	}
	// Precondition: the Symptom table named exactly trig:ManaAdded, so once
	// that key is implemented nothing else may hold the card back.
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	if m := r.Unsupported(sun, sup); len(m) != 0 {
		t.Fatalf("Caged Sun counted unsupported: %v", m)
	}
}
