package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsWatchClassSelfDefer: a source's own deferred TapsForMana watcher
// (Forbidden Orchard's "whenever you tap CARDNAME for mana ...") classes that
// source as deferred. tapsWatchClassForeign used to skip every TfSelf line
// (only the self Pain-class line, City of Brass, had its own pricing), so a
// lowered self line never reported deferSrc on the general planner path and
// the source stayed an ordinary census unit the kernel's search never
// deferred. The line matches only its own permanent, and because its
// controller is the payer it is no foreign ask: foreignOnly must stay false
// or tapsDeferDead would leave the source out of every payment.
func TestTapsWatchClassSelfDefer(t *testing.T) {
	tbl, ids := kcapsTable(t,
		kop.CardDef{Name: "Test Self TapsForMana", Types: kop.TLand,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigTaps, Flags: kop.TfSelf, Flags5: kop.Tf5TapsForMana, TapPlan: kop.TapPlanDefer}}}},
		kop.CardDef{Name: "Test Other Land", Types: kop.TLand},
	)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	k := testGen(sc)
	k.gen.anyTapsDefer = true
	k.gen.setFlag(ids[0], gfTapsDefer)
	src := sc.slot(sc.perm(ids[0], 0))
	other := sc.slot(sc.perm(ids[1], 0))

	pain, deferSrc, foreignOnly := sc.s.tapsWatchClassForeign(k, 0, src)
	if !deferSrc {
		t.Fatal("the source's own deferred TapsForMana line does not defer its tap")
	}
	if pain != 0 || foreignOnly {
		t.Fatalf("class = pain %d, foreignOnly %v; want 0, false (the payer's own line, no ask of another player)", pain, foreignOnly)
	}
	if !sc.s.tapsDeferred(k, 0, src) {
		t.Fatal("tapsDeferred does not read the self line's Defer class")
	}
	if _, d, _ := sc.s.tapsWatchClassForeign(k, 0, other); d {
		t.Fatal("the self line classed another permanent's tap")
	}
}
