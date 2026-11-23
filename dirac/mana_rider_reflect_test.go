package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// riderReflectScen is the manarider Gemstone Mine beside a second permanent
// (Mana Flare's reflected StatTapExtra) on the battlefield; each rewrites the
// Mine's rider ability when non-nil.
func riderReflectScen(t *testing.T, each func(a *kop.Ability)) (*scen, Slot, *kop.Ability) {
	t.Helper()
	tbl, err := cardsettest.Set(t, "manarider").Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
	ab.Static.Filter = kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
	fc := &tbl.Cards[id["Forest"]]
	fc.Abilities = append(append([]kop.Ability(nil), fc.Abilities...), ab)
	if each != nil {
		gc := &tbl.Cards[id["Gemstone Mine"]]
		gc.Abilities = append([]kop.Ability(nil), gc.Abilities...)
		for i := range gc.Abilities {
			if gc.Abilities[i].ManaRider == kop.ManaRiderDeplete {
				each(&gc.Abilities[i])
			}
		}
	}
	sc := newScenTable(t, tbl, Config{})
	sc.perm(id["Forest"], 0)
	mine := sc.perm(id["Gemstone Mine"], 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	sl := sc.slot(mine)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Gemstone Mine has no rider ability")
	}
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
	sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 3)
	sc.s.perms[sl].flags &^= pfTapped
	return sc, sl, a
}

// TestRiderFloatFoldsReflectedPick: a choice rider's tap beside a reflected
// StatTapExtra carrier floats the carrier's mana as one more of the colour the
// rider made, with no stall (addExtraCol); an all-at-once rider's pick is
// unasked, so floating it still stalls.
func TestRiderFloatFoldsReflectedPick(t *testing.T) {
	t.Run("choice", func(t *testing.T) {
		sc, sl, a := riderReflectScen(t, nil)
		if pc := sc.s.riderPC(sc.k, sl, a); !pc.manaTap || sc.s.srcReflectPick(sc.k, sl, pc) != 1 || a.ManaEach {
			t.Fatalf("precondition: a choice rider tap with a reflected extra (manaTap=%v each=%v)", pc.manaTap, a.ManaEach)
		}
		if !sc.s.riderGap(sc.k, 0, sl, 2) { // B
			t.Fatal("activation refused")
		}
		if sc.s.g.stalled == StallTapExtra {
			t.Fatal("the reflected pick stalled the float")
		}
		if got := sc.s.pl[0].pool[kop.B]; got != 2 {
			t.Fatalf("{B} in pool %d, want 2 (the rider's and the reflected one)", got)
		}
	})
	t.Run("each", func(t *testing.T) {
		sc, sl, a := riderReflectScen(t, func(a *kop.Ability) { a.ManaEach, a.ManaColors = true, kop.ColB|kop.ColG })
		if !a.ManaEach {
			t.Fatal("precondition: the rider is all-at-once")
		}
		if !sc.s.riderGap(sc.k, 0, sl, 0) {
			t.Fatal("activation refused")
		}
		if sc.s.g.stalled != StallTapExtra {
			t.Fatal("an all-at-once rider's reflected pick must stall")
		}
	})
}
