package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTrigDFightSupported pins the EDH fight-family class (ticket
// edh-trigD): Foe-Razer Regent (trig:Fight, with the TriggeredFighterLKICopy /
// DelayTriggerRememberedLKI bodies) and Neyith of the Dire Hunt
// (trig:FightOnce + trig:AttackerBlockedOnce) are primitive-complete under
// `make report`'s exact predicate (Registry.Unsupported against
// effects.Supported() with etbCopyShapeGate).
func TestEDHTrigDFightSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Foe-Razer Regent",
		"Neyith of the Dire Hunt",
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
