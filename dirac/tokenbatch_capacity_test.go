package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTokenBatchNearCapacityStalls: a phasing table's phased-out permanents
// keep their slots off occ but not free for enterNF's freeSlots branch. With
// occ one below the non-phasing nearCapacity line and five slots phased out,
// a ten-token batch must stall StallBoard instead of panicking "battlefield
// full".
func TestTokenBatchNearCapacityStalls(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Phasing Source", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{kop.ExtInstr(kop.ExtPhases)}}},
	})
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (createTokensHeld)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if !sc.k.phasing {
		t.Fatalf("the test table did not enable phasing")
	}
	for i := 0; i < MaxPerm; i++ {
		sc.perm(v.GrizzlyBears, 0)
	}
	if got := sc.s.occ.count(); got != MaxPerm {
		t.Fatalf("board holds %d permanents, want %d", got, MaxPerm)
	}
	for i := 0; i < 5; i++ {
		sc.s.phaseOut(sc.k, Slot(i), false)
	}
	if got := sc.s.occ.count(); got != MaxPerm-5 {
		t.Fatalf("after phasing out 5: occ %d, want %d", got, MaxPerm-5)
	}
	if free := sc.s.freeSlots().count(); free != 0 {
		t.Fatalf("free slots %d, want 0", free)
	}
	sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
	sc.s.exec(sc.k, &kop.Instr{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(10), C: int16(v.GrizzlyBears)})
	if got := sc.s.StallReason(); got != StallBoard {
		t.Fatalf("ten-token batch at a full phasing board: stall %q, want board", StallName(got))
	}
}
