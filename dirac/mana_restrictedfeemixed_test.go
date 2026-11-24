package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedFeeGenericSparesThePips: Castle Garenbrig's {2}{G}{G},{T}
// fee beside a spell whose own pips are another colour. Five Forests, two
// Islands and the Castle make eight mana of census; Inkwell Leviathan costs
// {7}{U}{U}, so only the fee alternative pays it (the Castle's six {G} are
// spent on creature spells only). The fee's generic is taken from the mana
// the spell's pips do not claim (CR 601.2h): taking it in kind order spent
// the two Islands' {U}, and the {U}{U} was left unpayable though both
// engines offered the cast.
func TestRestrictedFeeGenericSparesThePips(t *testing.T) {
	deck := []cardset.Entry{{Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "restrictedmana")
	for _, n := range []string{"Island", "Castle Garenbrig", "Inkwell Leviathan"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the restrictedmana table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	castle := sc.perm(id["Castle Garenbrig"], 0) // after the Forests: it enters tapped without one
	for i := 0; i < 2; i++ {
		sc.perm(id["Island"], 0)
	}
	if v, _, _ := sc.s.restrView(sc.k, 0, sc.slot(castle), sc.s.manaPCGen(sc.k, sc.slot(castle)),
		&payDesc{class: pdSpell, card: id["Inkwell Leviathan"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}); v == nil || restrFeeEmpty(v) {
		t.Fatal("the Castle's fee unit is not a unit of a creature spell's payment: the test does not reach the fee")
	}
	inkwell := sc.hand(id["Inkwell Leviathan"], 0)
	if !mayCast(sc, inkwell) {
		t.Fatalf("Inkwell Leviathan did not resolve (zone %d)", sc.s.objs[inkwell].zone)
	}
	if z := sc.s.objs[inkwell].zone; z != kop.ZoneBattlefield {
		t.Fatalf("Inkwell Leviathan is in zone %d, want the battlefield", z)
	}
	if sc.s.perms[sc.slot(castle)].flags&pfTapped == 0 {
		t.Fatal("the Castle is untapped: the fee alternative was not the payment")
	}
}

// TestTakeFeeGenericKeepsTheClaim: the one rule in isolation. The pool's
// green pays the fee's generic while the claimed {U}{U} stays; the claim
// itself is taken only when nothing else is left; an any-colour claim is
// one category held from the coloured kinds the pool has.
func TestTakeFeeGenericKeepsTheClaim(t *testing.T) {
	var pool [kop.NumManaKinds]uint8
	pool[kop.U], pool[kop.G] = 2, 3
	var claim [kop.NumManaKinds]int
	claim[kop.U] = 2
	p := pool
	takeFeeGeneric(&p, &claim, false, 2)
	if p[kop.U] != 2 || p[kop.G] != 1 {
		t.Fatalf("pool after the fee's generic = U%d G%d, want U2 G1", p[kop.U], p[kop.G])
	}
	p = pool
	takeFeeGeneric(&p, &claim, false, 4)
	if p[kop.G] != 0 || p[kop.U] != 1 {
		t.Fatalf("a generic larger than the surplus: pool U%d G%d, want U1 G0", p[kop.U], p[kop.G])
	}
	p = pool
	claim = [kop.NumManaKinds]int{}
	claim[kop.G] = 2 // any-colour: two coloured held, held from the first coloured kinds (U)
	takeFeeGeneric(&p, &claim, true, 3)
	if p[kop.U]+p[kop.G] != 2 || p[kop.U] != 2 {
		t.Fatalf("any-colour claim: pool U%d G%d, want U2 G0", p[kop.U], p[kop.G])
	}
}
