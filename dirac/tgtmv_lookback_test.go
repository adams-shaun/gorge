package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTargetMVKeepsXAfterCounter: Sound the Trumpets' shape (CR 608.2h,
// 202.3e). The resolving spell targets a spell on the stack, counters it, then
// reads Targeted$CardManaCost / MVOf(Target0): the mana value the target had
// on the stack, its announced X included, not the printed value CR 400.7 left
// after the move (gorge captureStackTargetSnapshots and
// count_ref_property.go's SpellManaValue arm; the kernel's snapTgtMV).
// Stargaze is {X}{B}{B}: its cast X was 4, so the stack mana value was
// 6 and the printed value 2.
func TestTargetMVKeepsXAfterCounter(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Stargaze", Count: 1}, {Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	ii := id["Stargaze"]
	if x := sc.k.t.Cards[ii].Cost.X; x != 1 {
		t.Fatalf("Stargaze cost has %d {X} symbols, want 1", x)
	}
	printed := int(sc.k.cards[ii].total)
	o := sc.hand(ii, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: o, card: ii, ctrl: 0, x: 4}
	e.tgt[0] = TCode(o)
	sc.s.push(e)
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, card: id["Valley Floodcaller"], ctrl: 0, nt: 1}, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.res.e.tgt[0] = TCode(o)
	ctx := sc.s.ctxEntry(&sc.s.res.e)
	sc.s.snapTgtMV(sc.k, &sc.s.res.e)
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != printed+4 {
		t.Fatalf("precondition: on the stack the live read = %d, want %d (printed %d + X 4)", got, printed+4, printed)
	}
	// The Counter has resolved: the target is off the stack.
	sc.s.objs[o].zone = kop.ZoneGraveyard
	if live := sc.s.objMV(sc.k, sc.s.objs[o].card, o); live != printed {
		t.Fatalf("postcondition: live mana value after the move = %d, want %d (X lost), else the snapshot is not what answers", live, printed)
	}
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != printed+4 {
		t.Fatalf("Targeted$CardManaCost after Counter = %d, want %d (X kept, CR 608.2h)", got, printed+4)
	}
	if got := sc.s.exprCold(sc.k, &kop.Expr{Kind: kop.ExMVOf, Ref: kop.RefTarget0}, ctx); got != printed+4 {
		t.Fatalf("ExMVOf(Target0) after Counter = %d, want %d", got, printed+4)
	}

	// The resolving trigger's object reads the same way (TriggeredCard$
	// CardManaCost; gorge's TriggerCard capture).
	sc2, id2 := newBLBScen(t, []cardset.Entry{{Name: "Stargaze", Count: 1}, {Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	ii2 := id2["Stargaze"]
	printed2 := int(sc2.k.cards[ii2].total)
	o2 := sc2.hand(ii2, 0)
	sc2.s.objs[o2].zone = kop.ZoneStack
	te := stackEntry{kind: seSpell, obj: o2, card: ii2, ctrl: 0, trigObj: o2, x: 4}
	sc2.s.push(te)
	sc2.s.res = resCtx{e: sc2.s.stack[sc2.s.sp-1], active: true, lastTok: NoObj, eachObj: NoObj}
	tctx := sc2.s.ctxEntry(&sc2.s.res.e)
	sc2.s.snapTgtMV(sc2.k, &sc2.s.res.e)
	if got := sc2.s.exprCold(sc2.k, &kop.Expr{Kind: kop.ExMVOf, Ref: kop.RefTriggerObj}, tctx); got != printed2+4 {
		t.Fatalf("TriggeredCard$CardManaCost on the stack = %d, want %d", got, printed2+4)
	}
	sc2.s.objs[o2].zone = kop.ZoneGraveyard
	if got := sc2.s.exprCold(sc2.k, &kop.Expr{Kind: kop.ExMVOf, Ref: kop.RefTriggerObj}, tctx); got != printed2+4 {
		t.Fatalf("TriggeredCard$CardManaCost after Counter = %d, want %d (X kept)", got, printed2+4)
	}

	// A rewritten target was not the snapshotted spell: the live read.
	vf := id["Valley Floodcaller"]
	other := sc.perm(vf, 1)
	sc.s.res.e.tgt[0] = TCode(other)
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != sc.k.t.Cards[vf].Cost.MV() {
		t.Fatalf("rewritten target reads %d, want the live printed value %d", got, sc.k.t.Cards[vf].Cost.MV())
	}
}
