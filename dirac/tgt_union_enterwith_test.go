package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestUnionReturnEnterWithAppliesOncePerEntrant: Relive the Past's "return up
// to one target artifact, one target land and one target creature card ... they
// are 5/5 Elementals" lowers to a union Return then a union OvEnterWith
// Override (CR 603.6a: the rider applies as each entrant enters, before the
// entry triggers match). Each entrant gets exactly one override application,
// not one more for every later target, and the "spent" count is back to zero after
// the union.
func TestUnionReturnEnterWithAppliesOncePerEntrant(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = []kop.Expr{{Kind: kop.ExRememberedCount}} // any expression turns the generated interpreter on
	tbl.Overrides = append([]kop.Override{}, tbl.Overrides...)
	ov := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{AddKw: kop.KwFlying})
	prog := []kop.Instr{
		{Op: kop.OpReturnEx, Obj: kop.RefTargets012, C: int16(kop.ZoneBattlefield)},
		{Op: kop.OpOverride, Obj: kop.RefTargets012, C: int16(ov), A: kop.Const(kop.DurPermanent), B: kop.Const(kop.OvEnterWith)},
	}
	var spell kop.CardDef
	spell.Name = "Test Relive"
	spell.Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}
	card := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef{}, tbl.Cards...), spell)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyUnion {
		t.Fatal("setup: the table's union ops did not switch the union path on")
	}
	gs := sc.grave(v.GrizzlyBears, 0, 3)
	e := &sc.s.res.e
	e.obj, e.ctrl, e.card, e.ab = gs[0], 0, card, 0
	for i, g := range gs {
		e.tgt[i], e.tver[i] = TCode(g), sc.s.objs[g].ver
		if sc.s.objs[g].zone != kop.ZoneGraveyard {
			t.Fatalf("setup: target %d is in zone %d, want the graveyard", i, sc.s.objs[g].zone)
		}
	}
	sc.s.res.bad = 0
	for sc.s.res.pc = 0; int(sc.s.res.pc) < len(prog); sc.s.res.pc++ {
		sc.s.exec(sc.k, &prog[sc.s.res.pc])
	}
	for i, g := range gs {
		if !sc.onBF(g) {
			t.Fatalf("target %d did not return", i)
		}
		n := 0
		sc.s.eachOvRec(sc.slot(g), func(r ovRec) {
			if int(r.idx) == ov+1 {
				n += r.reps()
			}
		})
		if n != 1 {
			t.Errorf("target %d carries the enter-with override %d times, want 1", i, n)
		}
	}
	if sc.s.g.ewDone != 0 || sc.s.g.inLoop {
		t.Errorf("after the union: ewDone %d inLoop %v, want 0 and false", sc.s.g.ewDone, sc.s.g.inLoop)
	}
}
