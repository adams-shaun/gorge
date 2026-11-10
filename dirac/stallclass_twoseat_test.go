package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoSeatRefusedOnlyPastTwo: a two-seat-only table plays two seats and
// stalls at its start with more.
func TestTwoSeatRefusedOnlyPastTwo(t *testing.T) {
	g := &genPrep{twoSeatOnly: true}
	if why := g.initStall(2); why != StallNone {
		t.Fatalf("two seats: initStall = %s, want none", StallName(why))
	}
	for _, n := range []uint8{3, 4} {
		if why := g.initStall(n); why != StallTwoSeat {
			t.Fatalf("%d seats: initStall = %s, want %s", n, StallName(why), StallName(StallTwoSeat))
		}
	}
	if why := (&genPrep{}).initStall(4); why != StallNone {
		t.Fatalf("an unmarked table at four seats: initStall = %s, want none", StallName(why))
	}
	if StallClassOf(StallTwoSeat) != StallCapacity {
		t.Fatal("StallTwoSeat is not a declared limit")
	}
}

// TestCardTwoSeat: the kgen mark and the single-opponent operands both make
// a card two-seat only; an ordinary program does not.
func TestCardTwoSeat(t *testing.T) {
	prog := func(in kop.Instr) kop.CardDef {
		return kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}}
	}
	plain := prog(kop.Instr{Op: kop.OpCreateTokenEx})
	if cardTwoSeat(&plain) {
		t.Fatal("a plain token maker is two-seat only")
	}
	marked := plain
	marked.TwoSeat = true
	att := prog(kop.Instr{Op: kop.OpCreateTokenEx, B: kop.ValueRef{N: kop.TokAttacking}})
	ret := prog(kop.Instr{Op: kop.OpReturnEx, B: kop.ValueRef{N: kop.RetUnderOpponent}})
	for name, c := range map[string]kop.CardDef{"marked": marked, "attacking token": att, "return under opponent": ret} {
		if !cardTwoSeat(&c) {
			t.Errorf("%s: not two-seat only", name)
		}
	}
	tb := &kop.Table{Cards: []kop.CardDef{plain, att}}
	g := &genPrep{}
	prepTwoSeat(tb, g)
	if !g.twoSeatOnly {
		t.Fatal("prepTwoSeat missed the attacking token")
	}
}
