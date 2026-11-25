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

// twoCarrierTable is eachReflectCast's table with a second Mana Flare-shaped
// carrier on "Judge of Currents" (two reflected picks, CR 106.12a), Birds
// rewritten to {W}{U}, and Firdoch Core costing w {W} and u {U}.
func twoCarrierTable(t *testing.T, w, u uint8) (*kop.Table, map[string]CardID) {
	t.Helper()
	return tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append([]kop.Ability(nil), j.Abilities...)
		tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
		for i := 0; i < 2; i++ {
			ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
			ab.Static.Filter = kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
			j.Abilities = append(j.Abilities, ab)
		}
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
		c := &tbl.Cards[id["Firdoch Core"]]
		c.Cost = kop.ManaCost{}
		c.Cost.Pips[kop.W], c.Cost.Pips[kop.U] = w, u
	})
}

// TestEachReflectedPickTwoCarriersAskTwice: two reflect carriers over an
// all-at-once {W}{U} source make {W}{U} and two independent picks (CR 106.12a).
// The planned tap asks the colour twice, in turn, and the answers complete the
// payment; the production's mana is floated at the first ask. (The plan prices
// the two picks as one colour twice, so a cost that needs two colours from them,
// {W}{W}{U}{U}, has no witness and stalls: the census below affords it.)
func TestEachReflectedPickTwoCarriersAskTwice(t *testing.T) {
	for _, tc := range []struct {
		name         string
		w, u         uint8
		first, again uint8
	}{
		{"{W}{W}{W}{U}: picks W then W", 3, 1, kop.W, kop.W},
		{"{W}{U}{U}{U}: picks U then U", 1, 3, kop.U, kop.U},
	} {
		tbl, id := twoCarrierTable(t, tc.w, tc.u)
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		sc.perm(id["Judge of Currents"], 0)
		b := sc.perm(id["Birds of Paradise"], 0)
		sl := sc.slot(b)
		if r := sc.s.srcEachPick(sc.k, sl, sc.s.manaPCGen(sc.k, sl)); r != 2 {
			t.Fatalf("%s: precondition: srcEachPick = %d, want 2", tc.name, r)
		}
		core := sc.hand(id["Firdoch Core"], 0)
		sc.kcastBuild(core, id["Firdoch Core"], 0)
		sc.s.bld().stage = bsMana
		if sc.s.g.stalled != 0 {
			t.Fatalf("%s: precondition: stalled %d before the payment", tc.name, sc.s.g.stalled)
		}
		modes := reflectAskOpts(t, sc)
		if sc.s.g.stalled != 0 || sc.s.ctl != ctlReflectPick {
			t.Fatalf("%s: stalled %d ctl %d, want the first colour ask", tc.name, sc.s.g.stalled, sc.s.ctl)
		}
		if len(modes) != 2 || modes[0] != kop.W || modes[1] != kop.U {
			t.Fatalf("%s: first ask options %v, want exactly W and U", tc.name, modes)
		}
		if sc.s.perms[sl].flags&pfTapped == 0 {
			t.Fatalf("%s: the source is not tapped when asked", tc.name)
		}
		if pool := sc.s.pl[0].pool; pool[kop.W] != 1 || pool[kop.U] != 1 {
			t.Fatalf("%s: pool %v at the first ask, want {W}{U} floating", tc.name, pool)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, tc.first))
		if sc.s.ctl != ctlReflectPick {
			t.Fatalf("%s: ctl = %d after the first answer, want the second ask", tc.name, sc.s.ctl)
		}
		if modes := reflectAskOpts(t, sc); len(modes) != 2 || modes[0] != kop.W || modes[1] != kop.U {
			t.Fatalf("%s: second ask options %v, want exactly W and U", tc.name, modes)
		}
		if pool := sc.s.pl[0].pool; pool[kop.W]+pool[kop.U] != 3 || pool[tc.first] < 2 {
			t.Fatalf("%s: pool %v at the second ask, want {W}{U} and the first pick floating", tc.name, pool)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, tc.again))
		if sc.s.ctl != ctlBuild {
			t.Fatalf("%s: ctl = %d after the second answer, want ctlBuild", tc.name, sc.s.ctl)
		}
		sc.s.Step()
		if sc.s.g.stalled != 0 {
			t.Fatalf("%s: stalled %d after the answers", tc.name, sc.s.g.stalled)
		}
		if sc.s.objs[core].zone != kop.ZoneStack {
			t.Fatalf("%s: the cast is in zone %d, want the stack (paid)", tc.name, sc.s.objs[core].zone)
		}
		if pool := sc.s.pl[0].pool; pool != [kop.NumManaKinds]uint8{} {
			t.Fatalf("%s: pool %v after the payment, want it spent exactly", tc.name, pool)
		}
	}
}

// TestEachReflectedPickTwoCarriersWindowStalls: the cast window keeps stalling
// with two carriers, as with one (TestEachReflectedPickIsNotPosedInTheWindow):
// gorge's harness answers the pick internally, so no ask is posed.
func TestEachReflectedPickTwoCarriersWindowStalls(t *testing.T) {
	var spell CardID
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append([]kop.Ability(nil), j.Abilities...)
		tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
		for i := 0; i < 2; i++ {
			ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
			ab.Static.Filter = kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
			j.Abilities = append(j.Abilities, ab)
		}
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
		spell = CardID(len(tbl.Cards))
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = 2, 1
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test WWU", Types: kop.TCreature, HasModeCost: true, Cost: c, Power: 1, Toughness: 1})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	birds := sc.perm(id["Birds of Paradise"], 0)
	sp := sc.hand(spell, 0)
	sl := sc.slot(birds)
	if r := sc.s.srcEachPick(sc.k, sl, sc.s.manaPCGen(sc.k, sl)); r != 2 {
		t.Fatalf("precondition: srcEachPick = %d, want 2", r)
	}
	sc.do(t, AkCast, TCode(sp), scAny)
	for i := 0; i < 50 && sc.s.g.stalled == 0; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenManaChoice {
			t.Fatalf("the window posed the reflected pick")
		}
		if d.Kind == DkPriority {
			break
		}
	}
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("stalled %d, want StallTapExtra", sc.s.g.stalled)
	}
	if !sc.tapped(birds) {
		t.Error("precondition: the window did not tap the source")
	}
}

// TestEachReflectedPickTwoCarriersCensus: the census counts each carrier's
// pick as one more mana of one of the produced types (srcEachPick = 2), so
// {W}{U} and two picks afford {W}{W}{U}{U} and {W}{W}{W}{U}, not four of a type.
func TestEachReflectedPickTwoCarriersCensus(t *testing.T) {
	tbl, id := twoCarrierTable(t, 1, 1)
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	sl := sc.slot(sc.perm(id["Birds of Paradise"], 0))
	if r := sc.s.srcEachPick(sc.k, sl, sc.s.manaPCGen(sc.k, sl)); r != 2 {
		t.Fatalf("precondition: srcEachPick = %d, want 2", r)
	}
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if sc.s.g.stalled != 0 || a.total != 4 {
		t.Fatalf("the census stalled %d, total %d, want 4 ({W}{U} and two picks)", sc.s.g.stalled, a.total)
	}
	for _, tc := range []struct {
		w, u uint8
		want bool
	}{{2, 2, true}, {3, 1, true}, {1, 3, true}, {4, 0, false}, {0, 4, false}} {
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = tc.w, tc.u
		if got := a.affords(&c, pipMaskOf(&c), 0); got != tc.want {
			t.Errorf("{W}x%d {U}x%d: affords = %v, want %v", tc.w, tc.u, got, tc.want)
		}
	}
}
