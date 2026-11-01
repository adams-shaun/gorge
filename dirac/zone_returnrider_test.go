package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReturnRiderMissPlainReturn: an OpReturn's StaticEffect rider (the
// OpOverride right after it on the same ref) applies only to what that
// return moved. Brilliance Unleashed's "return it and it's a 3/3 Robot"
// is lowered as two plain returns and an override (DBReturn2): for an
// artifact creature the second return finds it already on the
// battlefield and must not animate it (CR 608.2c, 611.2c). Enduring
// Vitality has the same shape: return this card, then override it.
func TestReturnRiderMissPlainReturn(t *testing.T) {
	sc, id, _ := tokScen(t, Config{NoElide: true}, "Enduring Vitality")
	card := id["Enduring Vitality"]
	ab := &sc.k.t.Cards[card].Abilities[0]
	if ab.Kind != kop.AbTriggered || ab.Trig.Event != kop.TrigDies || len(ab.Prog) < 2 ||
		ab.Prog[0].Op != kop.OpReturn || ab.Prog[0].Obj != kop.RefTriggerObj ||
		ab.Prog[1].Op != kop.OpOverride || ab.Prog[1].Obj != kop.RefTriggerObj {
		t.Fatalf("Enduring Vitality's first ability is not return-then-override on the trigger object")
	}
	nrec := func(o Obj) (n int) {
		sc.s.eachOvRec(sc.slot(o), func(r ovRec) { n++ })
		return
	}
	// Miss: the card is already on the battlefield, so the return moves
	// nothing and the rider applies to nothing.
	bf := sc.perm(card, 0)
	sc.s.res = resCtx{e: stackEntry{kind: seTrigger, obj: bf, srcVer: sc.s.objs[bf].ver, card: card, ab: 0, ctrl: 0, trigObj: bf}, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.ctl = ctlPriority
	sc.s.runProg(sc.k)
	if z := sc.s.objs[bf].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the missed return left the card in zone %d, want battlefield", z)
	}
	if n := nrec(bf); n != 0 {
		t.Fatalf("a return that moved nothing gave the card %d override records, want 0", n)
	}
	// Hit: the card is in the graveyard; the return puts it onto the
	// battlefield and the rider gives it exactly one record.
	gy := sc.obj(card, 0)
	sc.s.place(gy, kop.ZoneGraveyard, false)
	sc.s.res = resCtx{e: stackEntry{kind: seTrigger, obj: gy, srcVer: sc.s.objs[gy].ver, card: card, ab: 0, ctrl: 0, trigObj: gy}, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.ctl = ctlPriority
	sc.s.runProg(sc.k)
	if z := sc.s.objs[gy].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the return left the card in zone %d, want battlefield", z)
	}
	if n := nrec(gy); n != 1 {
		t.Fatalf("a return that moved the card gave it %d override records, want 1", n)
	}
}
