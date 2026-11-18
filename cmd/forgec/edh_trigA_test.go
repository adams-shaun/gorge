package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTriggerModesASupported pins the coverage census for the trigger
// modes Countered, MilledOnce and AbilityResolves: every card must be counted
// playable by the report predicate (primitive census plus the ETB-copy shape
// gate).
func TestEDHTriggerModesASupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{
		"Baral, Chief of Compliance",
		"Zellix, Sanity Flayer",
		"Tom Bombadil",
		"Narci, Fable Singer",
	} {
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
