package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMaxHandBelowSeven: a numeric maximum hand size below seven
// (StatNoMaxHandSize, Static.A = N+1) is enforced at cleanup: with a
// maximum of two, a hand of five discards three; the carrier's controller
// only, and with the carrier gone the maximum is seven again.
func TestMaxHandBelowSeven(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn}) // a generated table: k.gen != nil
	carrier := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Hand Cap", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatNoMaxHandSize, A: 3}}}})
	for _, tc := range []struct {
		name  string
		owner uint8 // controller of the carrier; 2: none
		hand  int
		want  int // discards the active player (seat 0) must make
	}{
		{"own cap 2, hand 5", 0, 5, 3},
		{"own cap 2, hand 2", 0, 2, 0},
		{"opponent's cap", 1, 5, 0},
		{"no carrier, hand 8", 2, 8, 1},
		{"no carrier, hand 7", 2, 7, 0},
	} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyHandSize {
			t.Fatal("anyHandSize not prepped")
		}
		if tc.owner < 2 {
			sc.perm(carrier, tc.owner)
		}
		for i := 0; i < tc.hand; i++ {
			sc.hand(v.Plains, 0)
		}
		sc.s.step = kop.StepCleanup
		sc.s.ctl = ctlStepBegin
		sc.s.Step()
		got := 0
		if sc.s.ctl == ctlCleanupDiscard {
			got = int(sc.s.discardLeft)
		}
		if got != tc.want {
			t.Errorf("%s: discardLeft %d, want %d", tc.name, got, tc.want)
		}
		if got > 0 && sc.s.PicksLeft() != got {
			t.Errorf("%s: PicksLeft %d, want discardLeft %d", tc.name, sc.s.PicksLeft(), got)
		}
	}
}

// TestParkNCPlayerProtection: two damage-doubling replacements of the
// source's controller would park a noncombat event to the opponent (an
// order ask), but protection from each opponent prevents the damage before
// the replacements (gorge emit), so parkNC deals it now (false) and
// replDamage prevents it all.
func TestParkNCPlayerProtection(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn}) // a generated table: k.gen != nil
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Tyrant", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoubleDamageToOpps}}}},
		kop.CardDef{Name: "Test Virtue", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatPlayerHexproof, A: 1}}}})
	tyrant, virtue := CardID(len(tbl.Cards)-2), CardID(len(tbl.Cards)-1)
	for _, prot := range []bool{false, true} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		src := sc.slot(sc.perm(tyrant, 0))
		sc.perm(tyrant, 0)
		if prot {
			sc.perm(virtue, 1)
		}
		parked := sc.s.parkNC(sc.k, 0, src, NoCard, uint8(PlayerTarget0+1), 3, 0)
		if parked == prot {
			t.Errorf("protected %v: parked %v, want %v", prot, parked, !prot)
		}
		if prot {
			if m := sc.s.replDamage(sc.k, 0, src, NoCard, uint8(PlayerTarget0+1), false, 3); m != 0 {
				t.Errorf("protected: replDamage %d, want 0", m)
			}
		}
	}
}
