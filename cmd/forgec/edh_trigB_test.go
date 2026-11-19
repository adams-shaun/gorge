package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTriggerModesBSupported pins the EDH trigger-modes class (ticket
// edh-trigB) for the cards this ticket finished: Toralf, God of Fury
// (trig:ExcessDamage), Font of Agonies (trig:PayLife) and Widespread Panic
// (trig:Shuffled). The class's remaining carriers -- Biomancer's Familiar,
// Cobra Trap, Karmic Justice, Foe-Razer Regent, Neyith of the Dire Hunt,
// Hibernation's End -- stay unsupported pending their split tickets (causer
// provenance, fight provenance, the CanAdapt static, the cumulative-upkeep
// payment marker) and are deliberately NOT listed here. Every card listed
// must be primitive-complete under `make report`'s exact predicate
// (Registry.Unsupported against effects.Supported() with etbCopyShapeGate).
func TestEDHTriggerModesBSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Toralf, God of Fury",
		"Font of Agonies",
		"Widespread Panic",
	} {
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
