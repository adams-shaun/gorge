package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachReflectScen is tapReflectScen over an all-at-once {W}{U} source (the
// Birds rewritten to Azorius Chancery's "{T}: Add {W}{U}") beside a reflect
// carrier (Mana Flare).
func eachReflectScen(t *testing.T) (*scen, Slot) {
	t.Helper()
	sc, sl := tapReflectScen(t, func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
	if pc := sc.s.manaPCGen(sc.k, sl); pc.manaXv == [kop.NumManaKinds]uint8{} {
		t.Fatal("precondition: the rewritten Birds are no all-at-once source")
	}
	return sc, sl
}

// TestEachReflectedPickIsOneOfTheProducedTypes: an all-at-once source under a
// reflect carrier makes its whole production and one more mana of ONE of the
// types it produced (CR 106.12a; gorge's script route answers the carrier's
// colour ask over the produced types), so {W}{U} and a pick are payable, the
// pick of either produced type in turn, and nothing else is.
func TestEachReflectedPickIsOneOfTheProducedTypes(t *testing.T) {
	sc, sl := eachReflectScen(t)
	pc := sc.s.manaPCGen(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("srcEachPick = %d, want 1", r)
	}
	sc.s.g.stalled = 0
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if sc.s.g.stalled != 0 {
		t.Fatalf("the census stalled %d", sc.s.g.stalled)
	}
	if a.total != 3 {
		t.Fatalf("precondition: total = %d, want 3 ({W}{U} and the pick)", a.total)
	}
	cost := func(w, u, b, g int) *kop.ManaCost {
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U], c.Pips[kop.B], c.Pips[kop.G] = uint8(w), uint8(u), uint8(b), uint8(g)
		return &c
	}
	for _, tc := range []struct {
		name string
		c    *kop.ManaCost
		want bool
	}{
		{"{W}{W}{U}: the pick is W", cost(2, 1, 0, 0), true},
		{"{W}{U}{U}: the pick is U", cost(1, 2, 0, 0), true},
		{"{W}{U}: the production alone", cost(1, 1, 0, 0), true},
		{"{W}{W}{W}: three of one type", cost(3, 0, 0, 0), false},
		{"{U}{U}{U}: three of one type", cost(0, 3, 0, 0), false},
		{"{W}{U}{G}: a type it did not produce", cost(1, 1, 0, 1), false},
		{"{W}{U}{B}: a type it did not produce", cost(1, 1, 1, 0), false},
	} {
		if got := a.affords(tc.c, pipMaskOf(tc.c), 0); got != tc.want {
			t.Errorf("%s: affords = %v, want %v (total=%d)", tc.name, got, tc.want, a.total)
		}
	}
	if a.affords(cost(0, 0, 0, 0), 0, 3) != true {
		t.Errorf("{3} not payable from the three mana")
	}
}

// eachWinScen is eachReflectScen plus an {X}{W} creature in hand and three Plains behind
// the source in battlefield order: the planner prices the announced cast (so
// the autopay surface offers it) and gorge's CR 601.2g window (xWindowPay)
// answers with the first listed source, the all-at-once one.
func eachWinScen(t *testing.T, cfg Config) (*scen, Obj, Obj) {
	t.Helper()
	var spell CardID
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
		spell = CardID(len(tbl.Cards))
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = 2, 1
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test WWU", Types: kop.TCreature, HasModeCost: true, Cost: c, Power: 1, Toughness: 1})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], cfg)
	sc.perm(id["Judge of Currents"], 0)
	b := sc.perm(id["Birds of Paradise"], 0)
	return sc, b, sc.hand(spell, 0)
}

// TestEachReflectedPickAskedInTheWindow: gorge's cast window taps the all-at-once
// source and the carrier's reflected pick resolves as a colour ask over the
// produced types (CR 605.3b, 106.12a; engine/effects ManaReflected, candidates in
// W U B R G C order). The kernel's real window walk poses that ask (a
// ChGenManaChoice pause, W then U) with the production already floated and the
// source tapped, floats the picked mana on the answer and carries on; a probe
// or fork walk of the same tap still stalls and floats no pick.
func TestEachReflectedPickAskedInTheWindow(t *testing.T) {
	sc, birds, sp := eachWinScen(t, Config{NoElide: true})
	sl := sc.slot(birds)
	pc := sc.s.manaPCGen(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcEachPick = %d, want 1", r)
	}
	var wu [kop.NumManaKinds]uint8
	wu[kop.W], wu[kop.U] = 1, 1
	sc.do(t, AkCast, TCode(sp), scAny)
	var d Decision
	for i := 0; i < 50; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenManaChoice {
			break
		}
		if d.Kind == DkPriority || sc.s.g.stalled != 0 {
			t.Fatalf("the cast never posed the pick: kind %d stalled %d", d.Kind, sc.s.g.stalled)
		}
		t.Fatalf("unexpected decision kind %d sub %d", d.Kind, d.Sub)
	}
	if d.Kind != DkChoose || d.Sub != ChGenManaChoice {
		t.Fatalf("no pick ask: kind %d sub %d", d.Kind, d.Sub)
	}
	if d.N != 2 || d.Options[0] != MakeAction(AkChoose, 0, 0, uint8(kop.W)) || d.Options[1] != MakeAction(AkChoose, 0, 0, uint8(kop.U)) {
		t.Fatalf("the pick lists %v, want W then U", d.Options[:d.N])
	}
	if !sc.tapped(birds) {
		t.Error("the source is not tapped at the ask")
	}
	if got := sc.s.pl[0].pool; got != wu {
		t.Errorf("pool at the ask %v, want the production %v", got, wu)
	}
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %d at the ask", sc.s.g.stalled)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.W)))
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %d after the answer", sc.s.g.stalled)
	}
	wwu := wu
	wwu[kop.W]++
	if got := sc.s.pl[0].pool; got != wwu {
		t.Fatalf("pool after the pick %v, want the production and one more {W} %v", got, wwu)
	}
	for i := 0; i < 50; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		t.Fatalf("unexpected decision kind %d sub %d after the pick", d.Kind, d.Sub)
	}
	if sc.s.ctl != ctlPriority {
		t.Fatalf("the cast never came back to priority (ctl %d)", sc.s.ctl)
	}
	if sc.s.objs[sp].zone != kop.ZoneStack { // the cast went through
		t.Error("the spell is not on the stack")
	}
}

// TestEachReflectedPickStallsInAProbe: a walk that is no live cast's (a probe:
// no build, a census, a potential walk) taps the same source and still stalls on
// the pick, floating the production and none of the pick.
func TestEachReflectedPickStallsInAProbe(t *testing.T) {
	sc, birds, _ := eachWinScen(t, Config{NoElide: true})
	if sc.s.buildLive() {
		t.Fatal("precondition: a build is live")
	}
	var wu [kop.NumManaKinds]uint8
	wu[kop.W], wu[kop.U] = 1, 1
	var c kop.ManaCost
	c.Pips[kop.W], c.Pips[kop.U] = 1, 1
	sc.s.g.stalled = 0
	if ok, _ := sc.s.windowTapWalk(sc.k, 0, &c, &c, false, false, false, false, false, false, false); !ok {
		t.Fatal("the probe walk failed")
	}
	if !sc.tapped(birds) {
		t.Fatal("precondition: the walk did not tap the source")
	}
	if sc.s.g.stalled != StallTapExtra {
		t.Errorf("stalled %d, want StallTapExtra", sc.s.g.stalled)
	}
	if got := sc.s.pl[0].pool; got != wu {
		t.Errorf("pool %v, want the production %v and no pick", got, wu)
	}
}
