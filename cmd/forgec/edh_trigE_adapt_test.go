package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTrigEAdaptSupported pins Biomancer's Familiar (Mode$ Adapt trigger and
// the Mode$ CanAdapt static) as primitive-complete under `make report`'s exact
// predicate (Registry.Unsupported against effects.Supported() with
// etbCopyShapeGate).
func TestEDHTrigEAdaptSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	const name = "Biomancer's Familiar"
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
