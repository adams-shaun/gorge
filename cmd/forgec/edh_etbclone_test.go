package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhETBCloneCards is the EDH census class "ETBReplacement:Copy except riders
// the clone shape gate declines": every card's as-enters DB$ Clone body must
// be offered (effects.CloneETBOffered) so the gate no longer counts it
// unplayable.
var edhETBCloneCards = []string{
	"Spark Double",
	"Cursed Mirror",
	"Imposter Mech",
	"Vizier of Many Faces",
	"Wall of Stolen Identity",
	"Estrid's Invocation",
	"Vesuvan Shapeshifter",
	"Moritte of the Frost",
	"Machine God's Effigy",
	"Infinite Reflection",
}

func TestEDHETBCloneRidersSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range edhETBCloneCards {
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
