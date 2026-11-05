package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsWatchDefersSource: with Judge of Currents ("whenever a Merfolk you
// control becomes tapped", a gain-life body: kop.TapPlanDefer) out, Firdoch
// Core (a changeling artifact, an any-colour Merfolk {T} source) is a source gorge's
// planner never taps: a {W} payment taps the Birds of Paradise that entered after it,
// not the Core, of equal flexibility, that is first in battlefield order. With no other plan the
// second pass of plan taps the Core after all (the potential play's prefix).
func TestTapsWatchDefersSource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Judge of Currents", Count: 1}, {Name: "Firdoch Core", Count: 1}, {Name: "Birds of Paradise", Count: 1}, {Name: "Plains", Count: 19}}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	judge, core, birds, plains := id["Judge of Currents"], id["Firdoch Core"], id["Birds of Paradise"], id["Plains"]
	run := func(t *testing.T, withBirds bool) (coreTapped, birdsTapped bool, sc *scen) {
		sc = newScenTableLib(t, tbl, plains, Config{NoElide: true})
		if !sc.k.gen.anyTapsDefer || !sc.k.gen.has(judge, gfTapsDefer) {
			t.Fatalf("anyTapsDefer %v, Judge gfTapsDefer %v: want both", sc.k.gen.anyTapsDefer, sc.k.gen.has(judge, gfTapsDefer))
		}
		co := sc.perm(core, 0)
		sc.perm(judge, 0)
		var pl Obj
		if withBirds {
			pl = sc.perm(birds, 0)
		}
		if !sc.s.tapsDeferred(sc.k, 0, sc.slot(co)) {
			t.Fatal("precondition: the Core is not a deferred source under Judge of Currents")
		}
		c := cost(0, kop.W)
		sc.s.tapFor(sc.k, 0, &c, 0, NoObj)
		return sc.tapped(co), withBirds && sc.tapped(pl), sc
	}
	t.Run("an ordinary source pays", func(t *testing.T) {
		if ct, pt, _ := run(t, true); ct || !pt {
			t.Fatalf("Core tapped %v, Birds tapped %v: want the Birds alone", ct, pt)
		}
	})
	t.Run("no other plan taps it", func(t *testing.T) {
		if ct, _, _ := run(t, false); !ct {
			t.Fatal("the Core stayed untapped: the second pass must plan with the deferred source")
		}
	})
	t.Run("without the classification the Core is first", func(t *testing.T) {
		DebugNoTapsDefer = true
		defer func() { DebugNoTapsDefer = false }()
		sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
		co := sc.perm(core, 0)
		sc.perm(judge, 0)
		pl := sc.perm(birds, 0)
		c := cost(0, kop.W)
		sc.s.tapFor(sc.k, 0, &c, 0, NoObj)
		if !sc.tapped(co) || sc.tapped(pl) {
			t.Fatalf("Core tapped %v, Birds tapped %v: want the Core (equal flexibility: battlefield order)", sc.tapped(co), sc.tapped(pl))
		}
	})
}
