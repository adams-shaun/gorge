package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// unlessSacScen is a table with one artifact whose activated ability is
// Rakdos, Patron of Chaos' shape: "you may sacrifice two creatures; if you
// don't, draw a card" (an OpUnlessPaid PaySac of 2 over a creature filter).
func unlessSacScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{} // non-nil: the generated interpreter
	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Unless Rite", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpUnlessPaid, Obj: kop.RefController, A: kop.Const(2), C: kop.PaySac | 1<<4, F: f},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
		}}}})
	return newScenTable(t, &tbl, Config{NoElide: true}), rite
}

// TestUnlessSacPicksLeaveTogether: an unless-cost sacrifice of two
// permanents collects both picks before either leaves (CR 601.2h, gorge
// unlessflow: PaySacrificeCost emits the picks once all are chosen), so the
// second menu is read with the first pick still on the battlefield. Before
// the fix the first pick left at once, and an Aura's pick returned the
// creature it stole before the second menu (std-mkm pair 4996: Coerced to
// Kill and Faerie Snoop under Rakdos, Patron of Chaos).
func TestUnlessSacPicksLeaveTogether(t *testing.T) {
	sc, rite := unlessSacScen(t)
	var bears [3]Obj
	for i := range bears {
		bears[i] = sc.perm(v.GrizzlyBears, 0)
	}
	src := sc.perm(rite, 0)
	sc.s.stack[0] = stackEntry{kind: seAbility, obj: src, srcVer: sc.s.objs[src].ver, card: rite, ab: 0, ctrl: 0}
	sc.s.sp = 1
	sc.s.resolveTop(sc.k)

	d := sc.s.Step() // pay or decline
	if d.Sub != ChGenPayMode || d.N != 2 {
		t.Fatalf("first ask sub %d with %d options, want the pay/decline ask", d.Sub, d.N)
	}
	sc.s.Apply(d.Options[1]) // pay
	d = sc.s.Step()
	if d.Sub != ChSacrifice && d.Sub != ChGenPayPick || d.N != 3 {
		t.Fatalf("first pick sub %d with %d options, want the three bears", d.Sub, d.N)
	}
	first := Obj(d.Options[0].Source())
	sc.s.Apply(d.Options[0])
	if z := sc.s.objs[first].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the first pick is in zone %d after its pick, want the battlefield until the last pick", z)
	}
	d = sc.s.Step()
	if d.N != 2 {
		t.Fatalf("second menu has %d options, want the two bears left", d.N)
	}
	for _, a := range d.Options[:d.N] {
		if Obj(a.Source()) == first {
			t.Fatalf("the second menu lists the first pick %d again", first)
		}
	}
	second := Obj(d.Options[0].Source())
	sc.s.Apply(d.Options[0])
	for _, o := range []Obj{first, second} {
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("pick %d is in zone %d after the last pick, want the graveyard", o, z)
		}
	}
	if sc.s.pl[0].graveN != 2 {
		t.Fatalf("graveyard holds %d cards, want the two picks", sc.s.pl[0].graveN)
	}
}
