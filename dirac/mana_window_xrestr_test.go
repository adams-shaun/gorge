package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXWitnessSpendsRestrictedLeftover: the lowered {X} witness prices each X
// with the restricted floating mana the fixed pips and generic could not fold
// credited to the {X} (gorge's census counts the pool, CR 106.6: restricted
// mana may pay any part of a spell's cost, {X} included). A cast of {X}{X}{U}{U}
// with {U}{R} floating restricted to spells and seven untapped sources pays
// X = 3 over six taps, not seven.
func TestXWitnessSpendsRestrictedLeftover(t *testing.T) {
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant | kop.TSorcery, Ctrl: kop.CtrlYou})
	castOrAct := kop.FilterID(len(tbl.Filters) - 1)
	mkSrc := func(name string, col kop.ColorMask) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: col, ManaN: 1,
				ManaRestrict: castOrAct, ManaRestrictAb: castOrAct},
		}}
	}
	// Two untapped coloured sources and five plain ones: X = 3 taps six of the
	// seven, whichever the planner picks.
	isle := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, mkSrc("Test Isle", kop.ColU))
	peak := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, mkSrc("Test Peak", kop.ColR))
	usp := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, mkSrc("Test Blue Spring", kop.ColU))
	rsp := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, mkSrc("Test Red Spring", kop.ColR))
	xspell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Mathemagics", Types: kop.TSorcery,
		Cost: kop.ManaCost{X: 2, Pips: [kop.NumManaKinds]uint8{kop.U: 2}}})
	sc := newScenTableLib(t, &tbl, isle, Config{NoElide: true})
	s := sc.s
	// the full surface lowers the hand cast gorge's cast planner never prices
	// (mana_riderwitness_test.go: the autopay surface offers no {X} cast)
	s.cfg = s.cfg&^cfgGorgeAutopay | cfgGorgeLowering

	// The springs' {U} and {R} are already floating restricted, as earlier taps
	// left them.
	for _, e := range []struct {
		o   Obj
		col int
	}{{sc.perm(usp, 0), int(kop.U)}, {sc.perm(rsp, 0), int(kop.R)}} {
		sl := sc.slot(e.o)
		c := s.cardAt(sl)
		rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
		if !s.floatRestrAb(sc.k, 0, sl, rab, int(e.col), 1) {
			t.Fatalf("precondition: the %s's mana did not float restricted", sc.k.t.Cards[c].Name)
		}
		s.perms[sl].flags |= pfTapped // the tap that produced it
	}
	if s.restrN(0) != 2 || s.restrOf(0, int(kop.U)) != 1 || s.restrOf(0, int(kop.R)) != 1 {
		t.Fatalf("precondition: restricted %v, want one {U} and one {R}", s.rb[0])
	}

	srcs := []Obj{sc.perm(isle, 0), sc.perm(isle, 0)}
	for i := 0; i < 5; i++ {
		srcs = append(srcs, sc.perm(peak, 0))
	}
	for i, o := range srcs {
		if sc.tapped(o) {
			t.Fatalf("precondition: source %d entered tapped", i)
		}
	}
	spell := sc.hand(xspell, 0)
	sc.do(t, AkCast, TCode(spell), scAny)
	if d := s.Step(); d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	n := 0
	for _, o := range srcs {
		if sc.tapped(o) {
			n++
		}
	}
	if n != 6 {
		t.Fatalf("the witness tapped %d of the seven sources, want six: the restricted {U}{R} funds X = 3 (CR 106.6)", n)
	}
}
