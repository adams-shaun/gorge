package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHPilesSupported pins the TwoPiles/MultiplePiles shape class the EDH
// deck census listed: every card here must count playable under the coverage
// report's predicate (primitive keys plus etbCopyShapeGate).
//
// Raging River (LeftRightPile$ with Effect-object Animate bodies) is not in
// the list: its pile bodies need ValidCommand effect lookups and per-creature
// "Left"/"Right" pile labels this engine does not model.
func TestEDHPilesSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{
		"Make an Example", "Sauron's Ransom", "Phyrexian Portal",
		"Truth or Tale", "Whims of the Fates",
	} {
		card, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(card, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
