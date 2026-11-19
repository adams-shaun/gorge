package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhAPICards is the Commander-deck blocker class this ticket closes: every
// card whose compiled script carries an unregistered effect API (or, for
// Celestial Convergence, an unmodelled count head), measured by forgec
// edhcover's exact predicate — Registry.Unsupported against
// effects.Supported() with etbCopyShapeGate set.
var edhAPICards = []string{
	"Magus of the Balance",
	"Illicit Auction",
	"Mages' Contest",
	"Pain's Reward",
	"Aeon Engine",
	"Bridge from Below",
	"Celestial Convergence",
	"Faces of the Past",
	"Serene Master",
	// Turnabout carries the OTHER tap-or-untap spelling, the one-election
	// `DB$ TapOrUntapAll` (a 2026-11-13 registration that predates this
	// ticket). Pinned here so the TaporUntapAll registration (the same one-
	// election body, Forge folds the two spellings) cannot
	// unregister it again.
	"Turnabout",
}

// TestEDHAPIsSupported pins the class: each card above must be primitive-
// complete under the coverage report's own predicate, so a deck holding it is
// no longer blocked from full gorge support.
func TestEDHAPIsSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range edhAPICards {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s still unsupported: %v", name, m)
		}
	}
}
