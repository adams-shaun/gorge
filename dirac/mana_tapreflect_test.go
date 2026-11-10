package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// tapReflectScen builds a table whose Judge of Currents carries a reflected
// StatTapExtra over any permanent its controller has (Mana Flare's body), with
// the Birds of Paradise rewritten by birds when non-nil, and puts both on the
// battlefield. It returns the scenario and the Birds' slot.
func tapReflectScen(t *testing.T, birds func(a *kop.Ability)) (*scen, Slot) {
	t.Helper()
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		if birds != nil {
			rewriteMana(tbl, id["Birds of Paradise"], birds)
		}
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	b := sc.perm(id["Birds of Paradise"], 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	return sc, sc.slot(b)
}

// TestTapReflectMultiColourSourcePricesThePick: a reflect carrier over a plain
// any-colour source (Mana Flare over Birds of Paradise) is one more mana of the
// pick: the unit makes manaN+1 of the colour it picks, with no stall; an
// all-at-once source of several colours (a bounce land's W and U together)
// still stalls, as gorge defers it.
func TestTapReflectMultiColourSourcePricesThePick(t *testing.T) {
	sc, sl := tapReflectScen(t, nil)
	pc := sc.s.manaPCGen(sc.k, sl)
	if m := srcMask(pc); m&(m-1) == 0 {
		t.Fatalf("precondition: the Birds' mask %b is not a several-colour source", m)
	}
	if pc.manaXv != [kop.NumManaKinds]uint8{} {
		t.Fatalf("precondition: the Birds are all-at-once (%v)", pc.manaXv)
	}
	if r := sc.s.srcReflectPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("srcReflectPick = %d, want 1", r)
	}
	if y := sc.s.srcYieldAt(sc.k, sl, pc); y != srcYield(pc)+1 {
		t.Fatalf("srcYieldAt = %d, want %d", y, srcYield(pc)+1)
	}
	if xv := sc.s.srcExtra(sc.k, sl, pc); xv != [kop.NumManaKinds]uint8{} || sc.s.g.stalled == StallTapExtra {
		t.Fatalf("srcExtra = %v, stalled %d: a pick-priced extra is no fixed vector and no stall", xv, sc.s.g.stalled)
	}
	before := sc.s.pl[0].pool
	sc.s.addExtraCol(sc.k, 0, sl, pc, kop.R)
	after := sc.s.pl[0].pool
	for c := range after {
		want := before[c]
		if c == kop.R {
			want++
		}
		if after[c] != want {
			t.Fatalf("addExtraCol(R): pool %v -> %v, want exactly one more R", before, after)
		}
	}
	sc.s.addExtra(sc.k, 0, sl, pc)
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("addExtra with no colour known stalled %d, want StallTapExtra (%d)", sc.s.g.stalled, StallTapExtra)
	}

	each, esl := tapReflectScen(t, func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
	epc := each.s.manaPCGen(each.k, esl)
	if epc.manaXv == [kop.NumManaKinds]uint8{} {
		t.Fatal("precondition: the rewritten Birds are no all-at-once source")
	}
	if r := each.s.srcReflectPick(each.k, esl, epc); r != 0 {
		t.Fatalf("all-at-once srcReflectPick = %d, want 0", r)
	}
	each.s.srcExtra(each.k, esl, epc)
	if each.s.g.stalled != StallTapExtra {
		t.Fatalf("an all-at-once source under a reflect carrier stalled %d, want StallTapExtra (%d)", each.s.g.stalled, StallTapExtra)
	}
}
