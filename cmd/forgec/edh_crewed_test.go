package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHBecomesCrewedSupported pins the crew-ticket class: every corpus
// Vehicle whose "whenever this Vehicle becomes crewed" trigger (Mode$
// BecomesCrewed, CR 702.122) is modelled must be primitive-complete for the
// coverage report's shape gate, so the imported Commander decks holding one
// are counted fully supported. These are the ticket's Symptom table; a card
// this engine cannot yet finish must be named in the ticket's report, never
// dropped from this list silently.
func TestEDHBecomesCrewedSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range []string{
		"Mobilizer Mech",
		"Ghost Ark",
		"Mighty Servant of Leuk-o",
	} {
		card, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %s", name)
		}
		if m := r.Unsupported(card, sup); len(m) != 0 {
			t.Errorf("%s counted unsupported: %v", name, m)
		}
	}
}
