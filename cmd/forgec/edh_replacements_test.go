package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHReplacementEventsSupported pins the EDH deck-census class of cards
// that need a replacement effect over an event the engine already produces:
// Tekuthal, Inquiry Dominus (repl:Proliferate), Fear of Sleep Paralysis
// (repl:RemoveCounter) and Averna, the Chaos Bloom (repl:Cascade). Before the
// mechanism existed each reported its repl: key from cards.Registry.Unsupported.
func TestEDHReplacementEventsSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Tekuthal, Inquiry Dominus",
		"Fear of Sleep Paralysis",
		"Averna, the Chaos Bloom",
	} {
		card, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %s", name)
		}
		if m := r.Unsupported(card, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
