package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// manaUnlessTapXScen is the unless walk with a payer whose {1} window can only
// be funded by a tap-another-permanent source (Saruli Caretaker's shape): p1
// has the source and two untapped creatures, so the window must ask which
// creature to tap. p0 has one Rhystic-Cave-shaped chooser whose three black
// (devotion) would pay the {X}{X} Ballista if the unless went unpaid.
func manaUnlessTapXScen(t *testing.T) (sc *scen, chooser, tapsrc, mtn Obj, cands [2]Obj, ball Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	dev := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	chooserID := add(kop.CardDef{Name: "Test Unless Chooser", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaV: dev, ManaAskColour: true, ManaUnless: true}}})
	pipsID := add(kop.CardDef{Name: "Test Black Pips", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{2: 3}}})
	ballID := add(kop.CardDef{Name: "Test Ballista", Types: kop.TCreature, Cost: kop.ManaCost{X: 1}, Power: -1, Toughness: -1})
	// the tap-another source is no creature, so the two creatures are its
	// only candidates and the window has more than the one it needs
	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	tapID := add(kop.CardDef{Name: "Test Tap Another", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostTapX, N: 1, Filter: f},
			ManaColors: kop.ColAny, ManaN: 1}}})
	sc = newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	chooser = sc.perm(chooserID, 0)
	sc.perm(pipsID, 0)
	ball = sc.hand(ballID, 0)
	tapsrc = sc.perm(tapID, 1)
	// the Mountain is the source the payability probe sees, listed after the
	// tap-another source so the walk takes the source that asks a pick first
	mtn = sc.perm(v.Mountain, 1)
	cands[0] = sc.perm(v.GrizzlyBears, 1)
	cands[1] = sc.perm(v.GrizzlyBears, 1)
	return sc, chooser, tapsrc, mtn, cands, ball
}

// TestManaUnlessPayerTapAnotherPickFinishes: the payer's unless mana window
// pauses on a tap-another source's pick inside the cast build. The build must
// pose the ChWindowTap pick (not ask the payer pay/decline again), the answer
// must tap the creature and the source, charge the {1}, and finish the unless
// walk with the mana suppressed.
func TestManaUnlessPayerTapAnotherPickFinishes(t *testing.T) {
	sc, chooser, tapsrc, mtn, cands, ball := manaUnlessTapXScen(t)
	sc.do(t, AkCast, TCode(ball), scAny)

	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision %+v, want the announced X", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // {2}: one chooser of three black

	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenManaChoice || d.N != 5 {
		t.Fatalf("decision %+v, want the colour ask over WUBRG", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 0 {
		t.Fatalf("first ask %+v, want player 0's pay/decline", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // p0 declines
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 1 || d.N != 2 {
		t.Fatalf("second ask %+v, want player 1's pay/decline", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // p1 pays {1} through the tap-another source

	// The {1} window stops on the source's pick. It is a build, so the pick
	// must come from the build's dispatch (the failure this test pins: the
	// build asked pay/decline a second time instead).
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChWindowTap || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision %+v, want player 1's ChWindowTap pick over two creatures", d)
	}
	picked := Obj(d.Options[0].Source())
	if picked != cands[0] && picked != cands[1] {
		t.Fatalf("the pick offers %v, want one of the two candidates", picked)
	}
	sc.s.Apply(d.Options[0])

	for i := 0; i < 30; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}

	if !sc.tapped(tapsrc) {
		t.Fatal("the tap-another source is untapped: the {1} was not charged")
	}
	if sc.tapped(mtn) {
		t.Fatal("the Mountain was tapped: the window took it instead of resuming the pick")
	}
	if !sc.tapped(picked) {
		t.Fatal("the picked creature is untapped: the pick was not charged")
	}
	other := cands[0]
	if picked == cands[0] {
		other = cands[1]
	}
	if sc.tapped(other) {
		t.Fatal("both candidates are tapped, want only the picked one")
	}
	if z := sc.s.objs[ball].zone; z != kop.ZoneHand {
		t.Fatalf("spell in zone %d, want the hand: the paid unless suppressed the three black", z)
	}
	if !sc.tapped(chooser) {
		t.Fatal("the chooser is untapped: its {T} was not paid")
	}
	if c := sc.s.perms[sc.slot(chooser)].chosen; c != 255 {
		t.Fatalf("the chooser records colour %d after the paid unless, want cleared", c)
	}
	if got := sc.s.pl[0].pool; got[kop.B] != 0 {
		t.Fatalf("activator pool %v, want no black: the paid unless suppressed the mana", got)
	}
	if got := sc.s.pl[1].pool; got != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("payer pool %v, want empty: the floated mana paid the {1}", got)
	}
}
