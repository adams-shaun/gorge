package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEveryETBCloneShapeCardIsPlayable is the class census for the
// ETBReplacement:Copy DB$ Clone bodies the clone selector used to reject: each
// name must pass reg.Unsupported with etbCopyShapeGate as the ShapeGate.
func TestEveryETBCloneShapeCardIsPlayable(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{"Essence of the Wild", "Thunderbond Vanguard", "The Fourteenth Doctor"} {
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
