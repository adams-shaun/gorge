package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// commanderTable is vanilla's table with one fixture commander appended: a
// {2}{G} 2/2 creature, optionally with a "this spell costs {2} less"
// static on the card itself.
func commanderTable(reduce int16) (*kop.Table, CardID) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{} // a generated-table extension, as every real pool has
	cost := kop.ManaCost{Generic: 2}
	cost.Pips[kop.G] = 1
	cd := kop.CardDef{Name: "Test Commander", Cost: cost, Types: kop.TCreature, Colors: kop.ColG, Power: 2, Toughness: 2}
	if reduce != 0 {
		cd.Abilities = []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatReduceCost, Zone: kop.ZoneCommand, A: reduce}}}
	}
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, cd)
	return tbl, id
}

// newCommanderScen deals two 20-card decks with the commander at index 0, as
// Config.Commander seats it, then clears the opening hands.
func newCommanderScen(t *testing.T, tbl *kop.Table, cmdCard CardID, commander bool) *scen {
	t.Helper()
	var deck [20]CardID
	for i := range deck {
		deck[i] = v.Forest
	}
	deck[0] = cmdCard
	s := new(State)
	s.Init(tbl, deck[:], deck[:], 1, Config{NoElide: true, Commander: commander, Commanders: [MaxSeats][2]int16{{0, -1}, {0, -1}}})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn = 2
	s.active = 0
	s.step = kop.StepMain1
	s.prio = 0
	s.passes = 0
	s.ctl = ctlPriority
	return &scen{s: s, k: s.k()}
}

// commanderCastOffered runs the priority walk and reports whether p is
// offered a cast sourced from its commander.
func commanderCastOffered(sc *scen, p uint8) (Action, bool) {
	sc.s.prio = p
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	cmd := sc.s.pl[p].cmd
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(cmd)) {
			return a, true
		}
	}
	return 0, false
}

// castCommanderFromCommandZone answers the priority walk with the
// commander's cast and runs the build to the stack.
func castCommanderFromCommandZone(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			sc.buildToStack(t)
			return
		}
	}
	t.Fatalf("commander cast not offered: %v", d.Options[:d.N])
}

// returnCommanderToCommandZone drops the committed cast from the stack and
// puts the card back into the command zone, as a removal spell would later
// (the command-zone return itself is out of scope).
func returnCommanderToCommandZone(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	s := sc.s
	if s.objs[o].zone != kop.ZoneStack || s.sp == 0 || s.stack[s.sp-1].obj != o {
		t.Fatalf("commander is not the top spell: zone %v sp %d", s.objs[o].zone, s.sp)
	}
	s.sp--
	s.detach(o)
	s.objs[o].zone = kop.ZoneCommand
	s.ctl = ctlPriority
	s.prio = 0
	s.step = kop.StepMain1
}

// TestCommanderCastTax: a commander costing {2}{G} is offered at sorcery
// speed with three lands, not at instant speed, and its second cast from
// the command zone costs {2}{G} plus the {2} tax.
func TestCommanderCastTax(t *testing.T) {
	tbl, card := commanderTable(0)
	sc := newCommanderScen(t, tbl, card, true)
	s := sc.s
	o := s.pl[0].cmd
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("commander not seated in the command zone: obj %d zone %v", o, s.objs[o].zone)
	}
	for i := 0; i < 3; i++ {
		sc.perm(v.Forest, 0)
	}
	s.step = kop.StepUpkeep
	if _, ok := commanderCastOffered(sc, 0); ok {
		t.Fatal("commander offered at instant speed")
	}
	s.step = kop.StepMain1
	if _, ok := commanderCastOffered(sc, 0); !ok {
		t.Fatal("commander not offered at sorcery speed with three lands")
	}
	castCommanderFromCommandZone(t, sc, o)
	if s.pl[0].cmdCast != 1 {
		t.Fatalf("cmdCast %d after the cast, want 1", s.pl[0].cmdCast)
	}
	if s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("commander zone %v after the cast, want the stack", s.objs[o].zone)
	}
	returnCommanderToCommandZone(t, sc, o)
	for i := 0; i < 4; i++ {
		sc.perm(v.Forest, 0)
	}
	if _, ok := commanderCastOffered(sc, 0); ok {
		t.Fatal("second cast offered with four mana; the taxed cost is five")
	}
	sc.perm(v.Forest, 0)
	if _, ok := commanderCastOffered(sc, 0); !ok {
		t.Fatal("second cast not offered with five mana")
	}
	castCommanderFromCommandZone(t, sc, o)
	if s.pl[0].cmdCast != 2 {
		t.Fatalf("cmdCast %d after the second cast, want 2", s.pl[0].cmdCast)
	}
	untapped := 0
	for m := s.ctrlM[0]; m.any(); m = m.rest() {
		sl := m.first()
		if s.cardAt(sl) == v.Forest && s.perms[sl].flags&pfTapped == 0 {
			untapped++
		}
	}
	if untapped != 0 {
		t.Fatalf("second cast left %d of the five lands untapped: the tax was not charged", untapped)
	}
}

// TestCommanderTaxReducedWithTheTotal: the tax is an additional cost, part
// of the total cost a reduction applies to (CR 601.2f, 903.8). A {2}{G}
// commander whose own static takes {3} off costs {G} on its first cast, and
// on its second {2}{G} plus the {2} tax less {3}: {1}{G}, offered with two
// lands, not one, and charged both.
func TestCommanderTaxReducedWithTheTotal(t *testing.T) {
	tbl, card := commanderTable(3) // more than the whole {2} generic
	sc := newCommanderScen(t, tbl, card, true)
	s := sc.s
	o := s.pl[0].cmd
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("commander not seated in the command zone: obj %d zone %v", o, s.objs[o].zone)
	}
	sc.perm(v.Forest, 0)
	if _, ok := commanderCastOffered(sc, 0); !ok {
		t.Fatal("reduced first cast not offered with one land")
	}
	castCommanderFromCommandZone(t, sc, o)
	if s.pl[0].cmdCast != 1 {
		t.Fatalf("cmdCast %d after the cast, want 1", s.pl[0].cmdCast)
	}
	returnCommanderToCommandZone(t, sc, o)
	sc.perm(v.Forest, 0)
	if _, ok := commanderCastOffered(sc, 0); ok {
		t.Fatal("second cast offered with one untapped land; want {1}{G}")
	}
	sc.perm(v.Forest, 0)
	if _, ok := commanderCastOffered(sc, 0); !ok {
		t.Fatal("second cast not offered with two untapped lands; the reduction takes the tax off too")
	}
	castCommanderFromCommandZone(t, sc, o)
	untapped := 0
	for m := s.ctrlM[0]; m.any(); m = m.rest() {
		sl := m.first()
		if s.cardAt(sl) == v.Forest && s.perms[sl].flags&pfTapped == 0 {
			untapped++
		}
	}
	if untapped != 0 {
		t.Fatalf("second cast left %d of the two untapped lands untapped: want {1}{G} charged", untapped)
	}
}

// TestCommanderCastOffNoOffer: the same board without Config.Commander offers
// nothing from the command zone.
func TestCommanderCastOffNoOffer(t *testing.T) {
	tbl, card := commanderTable(0)
	sc := newCommanderScen(t, tbl, card, false)
	s := sc.s
	if s.pl[0].cmd != NoObj {
		t.Fatalf("Commander off still seated a commander: obj %d", s.pl[0].cmd)
	}
	o := sc.obj(card, 0)
	s.objs[o].zone = kop.ZoneCommand
	for i := 0; i < 5; i++ {
		sc.perm(v.Forest, 0)
	}
	s.prio = 0
	var d Decision
	s.priorityOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			t.Fatalf("Commander off offered a cast from the command zone: %v", a)
		}
	}
}
