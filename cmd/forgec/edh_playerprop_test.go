package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHPlayerPropertySupported pins the PlayerCount<Group>$<Property> class
// of the Commander-deck coverage census: each card below read a count head
// the evaluator did not model, so cards.Registry.Unsupported (the predicate
// `make report` and `forgec edhcover` share, shape gate included) counted it
// unplayable. Every card here must now report no missing primitive.
func TestEDHPlayerPropertySupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{
		"Master of Waves",
		"Invasion of Fiora",
		"Decoy Gambit",
		"Tempting Contract",
		"Faramir, Prince of Ithilien",
		"Garland, Royal Kidnapper",
		"Simulacrum",
		"Tainted Sigil",
		"Celestial Convergence",
	} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s still unsupported, missing %v", name, m)
		}
	}
}
