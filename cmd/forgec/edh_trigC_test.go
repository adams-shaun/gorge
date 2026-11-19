package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTrigCDestroyedSupported pins the Mode$ Destroyed carriers of the
// EDH trigger-modes class (ticket edh-trigC): Karmic Justice and Cobra Trap
// must be primitive-complete under `make report`'s exact predicate
// (Registry.Unsupported against effects.Supported() with etbCopyShapeGate).
// Jaws, Relentless Predator also carries the mode but is not asserted here.
func TestEDHTrigCDestroyedSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{"Karmic Justice", "Cobra Trap"} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %q", name)
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s: unsupported (primitive): %v", name, m)
		}
		r.ShapeGate = etbCopyShapeGate
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s: unsupported (gated): %v", name, m)
		}
		r.ShapeGate = nil
	}
}
