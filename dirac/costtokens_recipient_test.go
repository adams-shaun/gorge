package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnlessDrawTargetedPlayerDraws: an UnlessCost$ Draw<N/Player.targetedBy>'s
// W is the drawer, not the payer (gorge pay/unlessflow.go AdvanceUnless draws
// for the selected player; cost_restokens.go payDrawToken). The lowered
// instruction carries the parent's player target as W, and paying the cost
// draws that player's card.
func TestUnlessDrawTargetedPlayerDraws(t *testing.T) {
	sc, synth, _ := m11Scen(t, []kop.Instr{{
		Op:  kop.OpUnlessPaid,
		Obj: kop.RefController,
		A:   kop.Const(2),
		C:   kop.PayDraw | 1<<4,
		W:   kop.RefTarget0,
	}})
	src := sc.perm(synth, 0)
	m11Start(sc, synth, func(e *stackEntry) {
		e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, src, sc.s.objs[src].ver, synth, 0
		e.trigPl = 0
		e.nt, e.tgt[0], e.tver[0] = 1, PlayerCode(1), 0 // the targeted player is seat 1
	})
	d := sc.s.Step()
	if d.Sub != ChGenPayMode || d.N != 2 {
		t.Fatalf("unless ask sub %d with %d options, want the pay/decline ask", d.Sub, d.N)
	}
	sc.s.Apply(d.Options[1]) // pay
	if got := int(sc.s.pl[0].handN); got != 0 {
		t.Fatalf("the payer drew %d cards, want 0: the targeted player is the drawer", got)
	}
	if got := int(sc.s.pl[1].handN); got != 2 {
		t.Fatalf("the targeted player drew %d cards, want 2", got)
	}
}
