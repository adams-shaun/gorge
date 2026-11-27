package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// loyaltyFlashTable is vanilla plus three planeswalker variants of Ember
// Walker: one carrying a self-scoped StatLoyaltyFlash static (Teferi, Master
// of Time's printed line), one whose grant also needs it to have entered this
// turn (The Wandering Emperor's Card.Self+ThisTurnEntered), and a granter with
// no loyalty abilities whose static reaches every planeswalker (no YouCtrl
// rider), to check the grant is its controller's alone (Caster$ You).
func loyaltyFlashTable(t *testing.T) (tbl *kop.Table, self, entered, wide CardID) {
	t.Helper()
	tb := *v.Table
	tb.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tb.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	fSelf := kop.FilterID(len(tb.Filters))
	tb.Filters = append(tb.Filters, kop.Filter{Flags: kop.FSelf})
	fEntered := kop.FilterID(len(tb.Filters))
	tb.Filters = append(tb.Filters, kop.Filter{Flags: kop.FSelf, Flags2: kop.F2EnteredThisTurn})
	fPw := kop.FilterID(len(tb.Filters))
	tb.Filters = append(tb.Filters, kop.Filter{Types: kop.TPlaneswalker})
	add := func(name string, f kop.FilterID, loyalty bool) CardID {
		w := v.Table.Cards[v.EmberWalker]
		w.Name = name
		if !loyalty {
			w = kop.CardDef{Name: name, Types: kop.TEnchantment}
		}
		w.Abilities = append(append([]kop.Ability(nil), w.Abilities...),
			kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatLoyaltyFlash, Filter: f}})
		tb.Cards = append(tb.Cards, w)
		return CardID(len(tb.Cards) - 1)
	}
	self = add("Test Instant Walker", fSelf, true)
	entered = add("Test Entered Walker", fEntered, true)
	wide = add("Test Wide Granter", fPw, false)
	return &tb, self, entered, wide
}

// loyaltyCan reports whether some loyalty ability of the permanent at sl
// passes canActivate at the given timing.
func loyaltyCan(sc *scen, sl Slot, sorc bool) bool {
	s, k := sc.s, sc.k
	var av avail
	s.availFor(k, s.perms[sl].ctrl, &av)
	cd := &k.t.Cards[s.cardAt(sl)]
	for ai := range cd.Abilities {
		a := &cd.Abilities[ai]
		if a.Kind == kop.AbActivated && a.Cost.Flags&kop.CostLoyalty != 0 && s.canActivate(k, sl, ai, a, sorc, &av) {
			return true
		}
	}
	return false
}

// TestInstantLoyaltyActivation: without the grant a loyalty ability is
// sorcery-speed only; under a live StatLoyaltyFlash static matching it the
// ability is offered at instant speed (with a spell on the stack, at every
// priority), CR 606.3's once-per-turn limit still holds, the entered-this-turn
// word ends the grant once the permanent is no longer new, and a grant
// controlled by the other player does not reach it (Caster$ You). The
// mutation (DebugLoyaltyFlashOff) restores the sorcery timing.
func TestInstantLoyaltyActivation(t *testing.T) {
	tbl, self, entered, wide := loyaltyFlashTable(t)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.uses.has(useLoyaltyFlash) {
		t.Fatal("precondition: the table does not arm the loyalty-timing grant")
	}
	plain := sc.perm(v.EmberWalker, 0)
	w := sc.perm(self, 0)
	e := sc.perm(entered, 0)
	pl, wl, el := sc.slot(plain), sc.slot(w), sc.slot(e)
	for _, sl := range []Slot{pl, wl, el} {
		if !loyaltyCan(sc, sl, true) {
			t.Fatalf("precondition: slot %d's loyalty ability is refused at sorcery speed", sl)
		}
	}
	if loyaltyCan(sc, pl, false) {
		t.Fatal("a loyalty ability with no grant passed at instant speed")
	}
	if !loyaltyCan(sc, wl, false) {
		t.Fatal("the self grant did not let its loyalty ability pass at instant speed")
	}
	if !sc.s.g.zoneTurn.has(e) || !loyaltyCan(sc, el, false) {
		t.Fatal("the entered-this-turn grant did not apply to a permanent that entered this turn")
	}
	sc.s.g.zoneTurn.clr(e)
	if loyaltyCan(sc, el, false) {
		t.Fatal("the entered-this-turn grant applied to a permanent that entered on an earlier turn")
	}

	// a priority with an object on the stack: the grant's loyalty ability is
	// offered, the plain one is not
	loyaltyActivatePush(t, sc, w)
	if sc.s.sp == 0 || sc.s.sorcerySpeed(0) {
		t.Fatal("precondition: the stack is empty after the activation")
	}
	if loyaltyOffered(sc, plain) {
		t.Fatal("the plain planeswalker's loyalty ability was offered with a spell on the stack")
	}
	if loyaltyOffered(sc, w) {
		t.Fatal("a second loyalty ability of the granted planeswalker was offered (CR 606.3)")
	}
	if !sc.s.loyaltyUsedAtLimit(sc.k, wl) || loyaltyCan(sc, wl, false) {
		t.Fatal("CR 606.3's once-per-turn limit did not hold under the grant")
	}

	old := DebugLoyaltyFlashOff
	DebugLoyaltyFlashOff = true
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	w2 := sc2.perm(self, 0)
	muted := loyaltyCan(sc2, sc2.slot(w2), false)
	DebugLoyaltyFlashOff = old
	if muted {
		t.Fatal("the mutation left the grant in force")
	}

	// the other player's grant over every planeswalker: Caster$ You is its
	// controller only
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	mine := sc3.perm(v.EmberWalker, 0)
	sc3.perm(wide, 1)
	if loyaltyCan(sc3, sc3.slot(mine), false) {
		t.Fatal("an opponent's loyalty-timing grant reached seat 0's planeswalker")
	}
	sc3.perm(wide, 0)
	if !loyaltyCan(sc3, sc3.slot(mine), false) {
		t.Fatal("the controller's own wide grant did not reach its planeswalker")
	}
}

// loyaltyActivatePush activates o's +1 targeting the opponent and leaves it
// on the stack, priority back with its controller.
func loyaltyActivatePush(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	sc.do(t, AkActivate, TCode(uint8(o)), scAny)
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
}
