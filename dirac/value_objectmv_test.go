package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSacrificedMV: Sacrificed$CardManaCost reads the sacrificed cost
// object's card from the entry's LKI record, after it left; 0 without one.
func TestSacrificedMV(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	if got := sc.s.sacrificedMV(sc.s.ctxEntry(&sc.s.res.e)); got != 0 {
		t.Fatalf("no LKI: %d", got)
	}
	sc.s.res.e.li = sc.s.snapLKI(a)
	sc.s.leave(sc.k, sc.slot(a), kop.ZoneGraveyard, false)
	if got, want := sc.s.sacrificedMV(sc.s.ctxEntry(&sc.s.res.e)), sc.k.t.Cards[fc].Cost.MV(); got != want || want == 0 {
		t.Fatalf("sacrificed MV %d, want %d", got, want)
	}

	// A copy's last known mana value is the copied card's (CR 707.2).
	cp := sc.perm(fc, 1)
	cpSl := sc.slot(cp)
	sc.s.perms[cpSl].ovr.flags |= ovCopy
	sc.s.perms[cpSl].ovr.copyOf = id["Plains"]
	sc.s.res.e.li = sc.s.snapLKI(cp)
	sc.s.leave(sc.k, cpSl, kop.ZoneGraveyard, false)
	if got := sc.s.sacrificedMV(sc.s.ctxEntry(&sc.s.res.e)); got != 0 {
		t.Fatalf("a copy of a Plains left with mana value %d, want 0", got)
	}

	// A face-down permanent's last known mana value is 0 (CR 708.2; gorge
	// captures state.SacrificedInfoOf.ManaValue while it is still face down).
	sc.k.gen.anyFaceDown = true // the BLB table has no face-down card; enable the mark
	d := sc.obj(fc, 0)
	dsl := sc.s.enterFaceDown(sc.k, d, 0, kop.MorphMorph)
	sc.s.res.e.li = sc.s.snapLKI(d)
	sc.s.leave(sc.k, dsl, kop.ZoneGraveyard, false)
	if got := sc.s.sacrificedMV(sc.s.ctxEntry(&sc.s.res.e)); got != 0 {
		t.Fatalf("face-down sacrificed MV %d, want 0", got)
	}
}

// TestTargetMV: Targeted$CardManaCost reads the chosen target's card on
// the battlefield and after it moved (the kernel keeps the object's id, as
// gorge does); 0 for a target illegal at resolution.
func TestTargetMV(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 1)
	want := sc.k.t.Cards[fc].Cost.MV()
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, card: fc, ctrl: 0, nt: 1, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.res.e.tgt[0] = TCode(a)
	ctx := sc.s.ctxEntry(&sc.s.res.e)
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != want || want == 0 {
		t.Fatalf("on the battlefield: %d, want %d", got, want)
	}
	sc.s.leave(sc.k, sc.slot(a), kop.ZoneGraveyard, false)
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != want {
		t.Fatalf("in the graveyard: %d, want %d", got, want)
	}
	sc.s.res.bad = 1
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != 0 {
		t.Fatalf("illegal target: %d", got)
	}
}

// TestRememberedMV: Remembered$CardManaCost sums the remembered objects'
// mana values, on the battlefield and off it.
func TestRememberedMV(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, h := sc.perm(fc, 0), sc.hand(fc, 0)
	sc.s.g.nrem = 0
	if got := sc.s.rememberedMV(sc.k); got != 0 {
		t.Fatalf("empty set: %d", got)
	}
	sc.s.remember(a)
	sc.s.remember(h)
	if got, want := sc.s.rememberedMV(sc.k), 2*sc.k.t.Cards[fc].Cost.MV(); got != want {
		t.Fatalf("remembered MV %d, want %d", got, want)
	}
}

// TestMVOfStackSpellCountsX: an object's mana value read by ExMVOf counts
// the announced X while it is a spell on the stack (CR 202.3e; gorge
// state.ManaValue), for the trigger object (TriggeredSpellAbility$
// CardManaCostLKI), a target and a remembered object. Stargaze is {X}{B}{B}.
func TestMVOfStackSpellCountsX(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Stargaze", Count: 1}, {Name: "Plains", Count: 8}})
	sg := id["Stargaze"]
	if x := sc.k.t.Cards[sg].Cost.X; x != 1 {
		t.Fatalf("Stargaze cost has %d {X} symbols, want 1", x)
	}
	printed := int(sc.k.cards[sg].total)
	o := sc.hand(sg, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: o, card: sg, ctrl: 0, trigObj: o, x: 3}
	e.tgt[0] = TCode(o)
	sc.s.push(e)
	ctx := evalCtx{src: NoObj, ctrl: 0, e: &sc.s.stack[sc.s.sp-1]}
	for _, tc := range []struct {
		name string
		ref  kop.Ref
	}{{"trigger object", kop.RefTriggerObj}, {"target", kop.RefTarget0}} {
		x := kop.Expr{Kind: kop.ExMVOf, Ref: tc.ref}
		if got := sc.s.exprCold(sc.k, &x, ctx); got != printed+3 {
			t.Fatalf("ExMVOf(%s) of a stack spell with X=3: %d, want %d", tc.name, got, printed+3)
		}
	}
	sc.s.g.nrem, sc.s.g.rem[0] = 1, o
	if got := sc.s.rememberedMV(sc.k); got != printed+3 {
		t.Fatalf("rememberedMV of a stack spell with X=3: %d, want %d", got, printed+3)
	}
	if got := sc.s.targetMV(sc.k, kop.RefTarget0, ctx); got != printed+3 {
		t.Fatalf("targetMV of a stack spell with X=3: %d, want %d", got, printed+3)
	}
}
