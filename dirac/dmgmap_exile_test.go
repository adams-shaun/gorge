package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDamageMapExileIfDiesMarksBeforeFlush is the dirac half of gorge's
// Burn the Accursed divergence (SUP-s186-supp/s189-supp p1.graveyard): the
// card's DealDamage carries both DamageMap$ True and ReplaceDyingDefined$
// Targeted, so the 5 damage is marked and flushed later by DB$
// DamageResolve. CR 614.1a/614.4: the exile-instead replacement must exist
// before the flush can kill the recipient, so the mark has to be registered
// while the instruction is marked, not when its map entry is dealt.
func TestDamageMapExileIfDiesMarksBeforeFlush(t *testing.T) {
	tbl := *v.NewTable()
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{{}}
	// The arena assigns mechExileEx from a table instruction carrying the
	// flag (arena.go), as Burn the Accursed's lowered DealDamage does.
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Exile Map Host", Types: kop.TCreature,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(1),
				C: int16(kop.DmgExileIfDiesThisTurn)}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	k := sc.k
	if k.gen.ix[mechExileEx] == arenaNone {
		t.Fatal("the table's exile-if-dies DealDamage did not give the arena a mechExileEx mask")
	}
	victim := sc.perm(v.TyphoidRats, 1) // 1/1
	sl := sc.slot(victim)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.tgt[0] = TCode(victim)
	marked := kop.Instr{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(5),
		C: int16(kop.DmgMapped | kop.DmgExileIfDiesThisTurn)}
	sc.s.exec(k, &marked)
	if sc.s.g.ndmg != 1 {
		t.Fatalf("the marked damage was not collected: ndmg %d, want 1", sc.s.g.ndmg)
	}
	if !sc.onBF(victim) {
		t.Fatal("the marked damage was dealt before the map flush")
	}
	if !sc.s.mSl(k, mechExileEx, sl) {
		t.Fatal("a mapped DealDamage with the exile-instead flag did not mark its recipient")
	}
	sc.s.exec(k, &kop.Instr{Op: kop.OpResolveDamageMap})
	sc.s.sba(k)
	if sc.onBF(victim) {
		t.Fatal("the 5 damage did not kill the 1/1")
	}
	if got := sc.s.objs[victim].zone; got != kop.ZoneExile {
		t.Fatalf("the marked creature's zone after the lethal flush is %d, want exile (CR 614.1a, 614.4)", got)
	}
}
