package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSneakProvExpr: ExSneaked reads the resolving
// spell entry's trigPlSneak mark, or the source spell entry still under a
// resolving self cast trigger. A non-spell entry with no source spell on the
// stack, or a plain spell, takes the else arm.
func TestSneakProvExpr(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	x := &sc.k.t.Exprs
	*x = append(*x, kop.Expr{Kind: kop.ExSneaked, Sub: [2]kop.ValueRef{kop.Const(1), kop.Const(0)}})
	v := kop.ValueRef{K: kop.VExpr, N: int16(len(*x) - 1)}
	for _, tc := range []struct {
		name string
		e    *stackEntry
		want int
	}{
		{"sneak cast", &stackEntry{kind: seSpell, trigPl: trigPlSneak}, 1},
		{"plain cast", &stackEntry{kind: seSpell}, 0},
		{"non-spell with the mark", &stackEntry{kind: seTrigger, trigPl: trigPlSneak}, 0},
		{"no entry", nil, 0},
	} {
		got := sc.s.evalV(sc.k, v, evalCtx{e: tc.e})
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
	// A self cast trigger resolves above its spell, and gorge still reads the
	// spell's CastFlags: the read follows the trigger to the source spell
	// entry (same object) on the stack.
	const spellObj Obj = 7
	trig := &stackEntry{kind: seTrigger, obj: spellObj}
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: spellObj, trigPl: trigPlSneak}
	sc.s.sp = 1
	if got := sc.s.evalV(sc.k, v, evalCtx{e: trig}); got != 1 {
		t.Errorf("trigger above a sneak spell: got %d, want 1", got)
	}
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: spellObj}
	if got := sc.s.evalV(sc.k, v, evalCtx{e: trig}); got != 0 {
		t.Errorf("trigger above a plain spell: got %d, want 0", got)
	}
}

// TestSneakProvBattlefieldGuard: a battlefield read of F2Sneaked is the
// arena's mechSneaked mark: set when the permanent's spell was
// cast for its sneak cost, clear for a hard cast of the same card, and clear
// again once the permanent leaves and a new object takes the slot (CR 400.7).
func TestSneakProvBattlefieldGuard(t *testing.T) {
	sc, cr, _ := ksneakScen(t)
	if sc.k.gen == nil || sc.k.gen.ix[mechSneaked] == arenaNone {
		t.Fatal("the sneak scenario has no mechSneaked arena index")
	}
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	if sc.k.t.Filters[f].Flags2 != kop.F2Sneaked {
		t.Fatal("the scenario's last filter is not the F2Sneaked read")
	}
	reads := func(sc *scen, o Obj) bool {
		return sc.s.matchPermGen(sc.k, f, sc.slot(o), 0, NoSlot, layerFull)
	}
	// a sneak cast
	o := sc.hand(cr, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	if reads(sc, bear) {
		t.Fatal("a permanent that was never cast matched F2Sneaked")
	}
	ksneakToBlockers(t, sc, []Obj{bear}, nil)
	ksneakCast(t, sc, o, bear)
	sc.passBoth(t)
	if !sc.onBF(o) {
		t.Fatal("sneak creature not on the battlefield")
	}
	if !reads(sc, o) {
		t.Fatal("a sneak-cast permanent did not match F2Sneaked")
	}
	// the permanent blinks: a new object takes the slot, unmarked
	s := sc.s
	s.leave(sc.k, sc.slot(o), kop.ZoneExile, false)
	sc.enterAs(o, 0)
	if reads(sc, o) {
		t.Fatal("a re-entered permanent matched F2Sneaked")
	}
	// a hard cast of the same card
	hc, cr2, _ := ksneakScen(t)
	hc.lands(0, v.Plains, 2)
	o2 := hc.hand(cr2, 0)
	hc.castCard(t, o2)
	hc.resolveAll(t)
	if !hc.onBF(o2) {
		t.Fatal("hard-cast creature not on the battlefield")
	}
	if reads(hc, o2) {
		t.Fatal("a hard-cast permanent matched F2Sneaked")
	}
}

// TestSneakProvSelfCastFilter: a SpellCast self trigger's F2Sneaked filter
// matches only a spell cast for its sneak cost (selfCastFilterOK).
func TestSneakProvSelfCastFilter(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Flags2: kop.F2Sneaked})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	if !sc.s.selfCastFilterOK(sc.k, f, &stackEntry{kind: seSpell, trigPl: trigPlSneak}) {
		t.Fatal("a sneak cast did not match Card.Self+sneaked")
	}
	if sc.s.selfCastFilterOK(sc.k, f, &stackEntry{kind: seSpell}) {
		t.Fatal("a plain cast matched Card.Self+sneaked")
	}
	if sc.s.selfCastFilterOK(sc.k, f, &stackEntry{kind: seTrigger, trigPl: trigPlSneak}) {
		t.Fatal("a non-spell entry matched Card.Self+sneaked")
	}
}
