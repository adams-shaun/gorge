package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHStaticsASupported pins the coverage report for the static-mode class
// ticket edh-staticsA: every card below is primitive-complete under the
// report's own predicate (cards.Registry.Unsupported with the ETB-copy shape
// gate), so imported Commander decks holding one are no longer blocked by a
// missing stat: key.
func TestEDHStaticsASupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{
		"Strong, the Brutish Thespian",
		"Seeker of Slaanesh",
		"Trove of Temptation",
		"Altar of the Pantheon",
		"Trinisphere",
		"Krark's Thumb",
		"River Song",
	} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus is missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
