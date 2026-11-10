package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// pickTestCard appends a vanilla creature to tbl and returns its id.
func pickTestCard(tbl *kop.Table, name string, mv int, power int8) CardID {
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: name, Types: kop.TCreature, Cost: kop.ManaCost{Generic: uint8(mv)},
		Power: power, Toughness: power,
	})
	return id
}

// pickTestFilter interns a pick filter and returns its id; filter 0 stays the
// empty filter the table requires.
func pickTestFilter(tbl *kop.Table, f kop.Filter) kop.FilterID {
	tbl.Filters = append(tbl.Filters, f)
	return kop.FilterID(len(tbl.Filters) - 1)
}

// pickTestDigSpell appends a costless sorcery whose body is one Dig op and
// returns its id.
func pickTestDigSpell(tbl *kop.Table, f kop.FilterID, name string) CardID {
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(3), B: kop.Const(2),
			C: int16(kop.ZoneHand), F: f,
		}}},
	}})
	return id
}

// pickTestLibScen stocks seat 0's library top to bottom with cs and returns
// the objects in that order. The vanilla Islands below them stay outside a
// three-card Dig window.
func pickTestLibScen(t *testing.T, tbl *kop.Table, cs ...CardID) (*scen, []Obj) {
	t.Helper()
	sc := newScenTableLib(t, tbl, v.Island, Config{NoElide: true})
	objs := make([]Obj, len(cs))
	for i, c := range cs {
		objs[i] = sc.obj(c, 0)
		sc.s.objs[objs[i]].zone = kop.ZoneLibrary
	}
	for i := len(objs) - 1; i >= 0; i-- {
		sc.s.pl[0].libTop(objs[i])
	}
	return sc, objs
}

// pickTestRun resolves spell's program directly and returns the decision it
// poses.
func pickTestRun(t *testing.T, sc *scen, spell CardID) Decision {
	t.Helper()
	sc.s.res = resCtx{
		e:      stackEntry{kind: seSpell, ctrl: 0, card: spell, ab: 0},
		active: true, lastTok: NoObj, eachObj: NoObj,
	}
	sc.s.ctl = ctlResolve
	sc.s.runProg(sc.k)
	return sc.s.Step()
}

// pickTestOption returns the option that chooses o.
func pickTestOption(t *testing.T, d *Decision, o Obj) Action {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkChoose && a.Source() == uint8(o) {
			return a
		}
	}
	t.Fatalf("object %d is not offered; options %v", o, d.Options[:d.N])
	return 0
}

// pickTestOffers reports whether the decision offers o.
func pickTestOffers(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Source() == uint8(o) {
			return true
		}
	}
	return false
}

// TestKernelPickSumBudget: a Dig whose filter head carries a budget of 4
// offers a 3-drop and then only a 1-drop (the 2-drop's mana value no longer
// fits the running sum). The bound is kop.Filter.PickSumMax, read by
// State.pickFits in the dig option builder and digHasPick.
func TestKernelPickSumBudget(t *testing.T) {
	tbl := v.NewTable()
	// a non-nil expression table makes prepareGen build the OpDigEx extension
	tbl.Exprs = []kop.Expr{}
	mv3 := pickTestCard(tbl, "Test Pick Three", 3, 3)
	mv2 := pickTestCard(tbl, "Test Pick Two", 2, 2)
	mv1 := pickTestCard(tbl, "Test Pick One", 1, 1)
	f := pickTestFilter(tbl, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneLibrary, PickSumMax: kop.Const(4)})
	spell := pickTestDigSpell(tbl, f, "Test Pick Sum Dig")
	// preconditions: the printed values are what the bound reads, and the
	// filter carries the budget
	for _, c := range []struct {
		id CardID
		mv int
	}{{mv3, 3}, {mv2, 2}, {mv1, 1}} {
		if got := tbl.Cards[c.id].Cost.MV(); got != c.mv {
			t.Fatalf("card %d mana value %d, want %d", c.id, got, c.mv)
		}
	}
	if tbl.Filters[f].PickSumMax != kop.Const(4) {
		t.Fatalf("filter %d budget %v, want 4", f, tbl.Filters[f].PickSumMax)
	}
	sc, objs := pickTestLibScen(t, tbl, mv3, mv2, mv1)
	d := pickTestRun(t, sc, spell)
	if d.Sub != ChGenDig {
		t.Fatalf("decision kind %d sub %d player %d opts %v, want a Dig ask", d.Kind, d.Sub, d.Player, d.Options[:d.N])
	}
	for _, o := range objs {
		if !pickTestOffers(&d, o) {
			t.Fatalf("object %v not offered before any pick; options %v", o, d.Options[:d.N])
		}
	}
	sc.s.Apply(pickTestOption(t, &d, objs[0]))
	d = sc.s.Step()
	if !pickTestOffers(&d, objs[2]) {
		t.Fatalf("1-drop not offered after the 3-drop; options %v", d.Options[:d.N])
	}
	if pickTestOffers(&d, objs[1]) {
		t.Fatalf("2-drop offered after the 3-drop: the running sum 5 exceeds the budget 4")
	}
}

// TestKernelPickDistinctPowers: WithDifferentPowers drops a second 2-power
// card once one is picked (kop.Filter.PickDistinctPower).
func TestKernelPickDistinctPowers(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	pow2a := pickTestCard(tbl, "Test Pick Power A", 2, 2)
	pow2b := pickTestCard(tbl, "Test Pick Power B", 2, 2)
	pow1 := pickTestCard(tbl, "Test Pick Power One", 1, 1)
	f := pickTestFilter(tbl, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneLibrary, PickDistinctPower: true})
	spell := pickTestDigSpell(tbl, f, "Test Pick Distinct Dig")
	// preconditions: the two 2-power cards differ in name but share a power,
	// and the filter asks for distinct powers
	if tbl.Cards[pow2a].Name == tbl.Cards[pow2b].Name || tbl.Cards[pow2a].Power != tbl.Cards[pow2b].Power {
		t.Fatal("the two 2-power cards are not a distinct-power pair")
	}
	if !tbl.Filters[f].PickDistinctPower {
		t.Fatalf("filter %d does not carry the distinct-power flag", f)
	}
	sc, objs := pickTestLibScen(t, tbl, pow2a, pow2b, pow1)
	d := pickTestRun(t, sc, spell)
	if d.Sub != ChGenDig {
		t.Fatalf("decision sub %d, want a Dig ask", d.Sub)
	}
	for _, o := range objs {
		if !pickTestOffers(&d, o) {
			t.Fatalf("object %v not offered before any pick; options %v", o, d.Options[:d.N])
		}
	}
	sc.s.Apply(pickTestOption(t, &d, objs[0]))
	d = sc.s.Step()
	if !pickTestOffers(&d, objs[2]) {
		t.Fatalf("1-power card not offered after a 2-power pick; options %v", d.Options[:d.N])
	}
	if pickTestOffers(&d, objs[1]) {
		t.Fatalf("a second 2-power card offered after a 2-power pick")
	}
}

// TestKernelPickSumBudgetChooseMove: the plain searchCands walk (a hidden
// graveyard ChooseMove) applies the same bound after a taken card.
func TestKernelPickSumBudgetChooseMove(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	mv3 := pickTestCard(tbl, "Test Grave Three", 3, 3)
	mv2 := pickTestCard(tbl, "Test Grave Two", 2, 2)
	mv1 := pickTestCard(tbl, "Test Grave One", 1, 1)
	f := pickTestFilter(tbl, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneGraveyard, PickSumMax: kop.Const(4)})
	spell := CardID(len(tbl.Cards))
	in := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(2), F: f, C: int16(kop.ZoneHand)}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grave Pick", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	grave := append(sc.grave(mv3, 0, 1), sc.grave(mv2, 0, 1)...)
	grave = append(grave, sc.grave(mv1, 0, 1)...)
	// preconditions: all three are in seat 0's graveyard, the walk's pool,
	// and the pick is the budgeted one
	for i, o := range grave {
		if sc.s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("card %d zone %d, want the graveyard", i, sc.s.objs[o].zone)
		}
	}
	d := pickTestRun(t, sc, spell)
	if d.Sub != ChGenHiddenPick {
		t.Fatalf("decision sub %d, want the hidden pick", d.Sub)
	}
	for _, o := range grave {
		if !pickTestOffers(&d, o) {
			t.Fatalf("object %v not offered before any pick; options %v", o, d.Options[:d.N])
		}
	}
	sc.s.Apply(pickTestOption(t, &d, grave[0]))
	d = sc.s.Step()
	if !pickTestOffers(&d, grave[2]) {
		t.Fatalf("1-drop not offered after the 3-drop; options %v", d.Options[:d.N])
	}
	if pickTestOffers(&d, grave[1]) {
		t.Fatalf("2-drop offered after the 3-drop: the running sum 5 exceeds the budget 4")
	}
}

// TestKernelPickDistinctForcedTake: a mandatory distinct-power Dig that takes
// every card still takes greedily -- the second 2-power card is skipped
// unasked -- as gorge's digGreedyTake does (digForced's no-budget arm).
func TestKernelPickDistinctForcedTake(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	pow2a := pickTestCard(tbl, "Test Forced Power A", 2, 2)
	pow2b := pickTestCard(tbl, "Test Forced Power B", 2, 2)
	pow1 := pickTestCard(tbl, "Test Forced Power One", 1, 1)
	f := pickTestFilter(tbl, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneLibrary, PickDistinctPower: true})
	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Forced Distinct Dig", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(3), B: kop.Const(3),
			C: int16(kop.ZoneHand), F: f,
		}}},
	}})
	sc, objs := pickTestLibScen(t, tbl, pow2a, pow2b, pow1)
	// precondition: three cards in the window, two sharing a power
	if got := sc.s.g.nlook; got != 0 {
		t.Fatalf("window %d before the dig, want 0", got)
	}
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, card: spell, ab: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.ctl = ctlResolve
	sc.s.runProg(sc.k)
	if sc.s.res.ask == raGenDig {
		t.Fatal("a forced take posed a choice")
	}
	if sc.s.objs[objs[0]].zone != kop.ZoneHand || sc.s.objs[objs[2]].zone != kop.ZoneHand {
		t.Fatalf("zones %d and %d, want both in the hand", sc.s.objs[objs[0]].zone, sc.s.objs[objs[2]].zone)
	}
	if sc.s.objs[objs[1]].zone != kop.ZoneLibrary {
		t.Fatalf("the second 2-power card zone %d, want it left in the library", sc.s.objs[objs[1]].zone)
	}
}
