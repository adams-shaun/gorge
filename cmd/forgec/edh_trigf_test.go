package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTrigFCumulativeSupported pins Hibernation's End (ticket edh-trigF)
// as primitive-complete under `make report`'s exact predicate
// (Registry.Unsupported against effects.Supported() with etbCopyShapeGate):
// the Mode$ PayCumulativeUpkeep trigger registers, so commander decks
// carrying the card stop blocking on import.
func TestEDHTrigFCumulativeSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	c, ok := r.Lookup("Hibernation's End")
	if !ok {
		t.Fatalf("corpus missing %q", "Hibernation's End")
	}
	if m := r.Unsupported(c, sup); len(m) != 0 {
		t.Errorf("Hibernation's End: unsupported (primitive): %v", m)
	}
	r.ShapeGate = etbCopyShapeGate
	if m := r.Unsupported(c, sup); len(m) != 0 {
		t.Errorf("Hibernation's End: unsupported (gated): %v", m)
	}
	r.ShapeGate = nil
}
