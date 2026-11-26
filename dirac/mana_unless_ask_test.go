package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// manaUnlessScen is askColScen with the colour-asking sources carrying the
// unless flag: a Rhystic-Cave-shaped source (kop.Ability.ManaAskColour |
// ManaUnless) is "{T}: choose a colour, add that much of it unless any player
// pays {1}". Two of them beside a black-pip permanent (devotion to black 3)
// and a {X}{X} Ballista; p1 has a Mountain for the {1}.
func manaUnlessScen(t *testing.T) (*scen, [2]Obj, Obj, Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	dev := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	chooser := add(kop.CardDef{Name: "Test Unless Chooser", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaV: dev, ManaAskColour: true, ManaUnless: true}}})
	pips := add(kop.CardDef{Name: "Test Black Pips", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{2: 3}}})
	ball := add(kop.CardDef{Name: "Test Ballista", Types: kop.TCreature, Cost: kop.ManaCost{X: 2}, Power: -1, Toughness: -1})
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	srcs := [2]Obj{sc.perm(chooser, 0), sc.perm(chooser, 0)}
	sc.perm(pips, 0)
	mtn := sc.perm(v.Mountain, 1)
	return sc, srcs, mtn, sc.hand(ball, 0)
}

// TestManaUnlessCaveAsksInSeatOrderAndOpponentPays: the colour window's
// unless cost asks every living player in seat order (gorge's UnlessPayers,
// AliveFrom(0)); the opponent's payment taps its Mountain for the {1}, the
// asked source adds no mana (its chain's Cleanup still clears the colour),
// and the cast reverses for want of the {X}s.
func TestManaUnlessCaveAsksInSeatOrderAndOpponentPays(t *testing.T) {
	sc, srcs, mtn, ball := manaUnlessScen(t)
	sc.do(t, AkCast, TCode(ball), scAny)

	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision %+v, want the announced X", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 3)) // {6}: two sources of three black

	// the first chooser's colour and unless asks
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenManaChoice || d.N != 5 {
		t.Fatalf("decision %+v, want the colour ask over WUBRG", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 0 || d.N < 1 {
		t.Fatalf("first ask %+v, want player 0's ask", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 1 || d.N != 2 {
		t.Fatalf("second ask %+v, want player 1's pay/decline", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // pay {1}: this source makes nothing

	// the second chooser's asks: both decline, so it makes three black
	d = sc.s.Step()
	if d.Sub != ChGenManaChoice {
		t.Fatalf("decision %+v, want the second colour ask", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
	for want := uint8(0); want < 2; want++ {
		d = sc.s.Step()
		if d.Sub != ChGenPayMode || d.Player != want {
			t.Fatalf("ask %+v, want player %d's pay/decline", d, want)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
	}

	for i := 0; i < 30; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[ball].zone; z != kop.ZoneHand {
		t.Fatalf("the cast's {6} was paid: spell in zone %d, want the hand (one source made nothing)", z)
	}
	if !sc.tapped(mtn) {
		t.Fatal("the payer's Mountain is untapped: the {1} was not charged")
	}
	if got := sc.s.pl[0].pool; got[kop.B] != 3 {
		t.Fatalf("activator pool %v, want the second chooser's three black left floating", got)
	}
	if !sc.tapped(srcs[0]) || !sc.tapped(srcs[1]) {
		t.Fatal("a chooser is untapped: its {T} was not paid before the ask")
	}
	for _, o := range srcs {
		if c := sc.s.perms[sc.slot(o)].chosen; c != 255 {
			t.Fatalf("a chooser records colour %d after the paid unless, want cleared", c)
		}
	}
}

// TestManaUnlessCaveAllDeclineAdds: when every player declines, each source
// adds its chosen colour and the cast is paid; the opponent's Mountain stays
// untapped.
func TestManaUnlessCaveAllDeclineAdds(t *testing.T) {
	sc, srcs, mtn, ball := manaUnlessScen(t)
	sc.do(t, AkCast, TCode(ball), scAny)

	d := sc.s.Step()
	if d.Sub != ChX {
		t.Fatalf("decision %+v, want the announced X", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 3))
	for range srcs {
		d = sc.s.Step()
		if d.Sub != ChGenManaChoice {
			t.Fatalf("decision %+v, want a colour ask", d)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
		for want := uint8(0); want < 2; want++ {
			d = sc.s.Step()
			if d.Sub != ChGenPayMode || d.Player != want {
				t.Fatalf("ask %+v, want player %d's pay/decline", d, want)
			}
			sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline
		}
	}
	for i := 0; i < 30; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[ball].zone; z != kop.ZoneStack {
		t.Fatalf("spell in zone %d, want the stack: the choosers' mana paid it", z)
	}
	if sc.tapped(mtn) {
		t.Fatal("the opponent's Mountain was tapped although it declined")
	}
	if !sc.tapped(srcs[0]) || !sc.tapped(srcs[1]) {
		t.Fatal("a chooser is untapped: its {T} was not paid")
	}
}
