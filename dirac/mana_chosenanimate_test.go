package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// wickermawScen: p0 has a hand-built Foraging Wickermaw (a {1} chosen-colour
// mana ability whose program is the chosen-colour Animate) and one colourless
// mana in the pool, enough for its fee. The fields mirror the card kgen lowers
// (TestDbManaWickermawLowers): CostOncePerTurn, one generic, ColAny, one
// ManaRiderChosenAnimate Override. The dirac package may not read a raw table
// (internal/archtest TestOnlyTheOracleReadsRawTables), so the lowered card
// itself is pinned in kgen and the kernel runs this matching shape.
func wickermawScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	tbl := v.NewTable()
	first := len(tbl.Overrides)
	for _, col := range [5]kop.ColorMask{kop.ColW, kop.ColU, kop.ColB, kop.ColR, kop.ColG} {
		tbl.Overrides = append(tbl.Overrides, kop.Override{HasColors: true, SetColors: col})
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Foraging Wickermaw", Types: kop.TArtifact | kop.TCreature, Power: 1, Toughness: 3,
		Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostOncePerTurn, Mana: kop.ManaCost{Generic: 1}},
			ManaColors: kop.ColAny, ManaN: 1, ManaRider: kop.ManaRiderChosenAnimate,
			Prog: []kop.Instr{{Op: kop.OpOverride, Obj: kop.RefSelf, A: kop.Const(kop.DurEOT),
				B: kop.Const(kop.OvChosenColor), C: int16(first)}},
		}},
	})
	id := CardID(len(tbl.Cards) - 1)
	sc := newScenTableLib(t, tbl, v.Plains, Config{NoElide: true})
	wick := sc.perm(id, 0)
	sc.s.pl[0].pool[kop.C] = 1
	return sc, wick
}

// TestWickermawChosenAnimateRider: the {1} rider adds one mana of the
// answer's colour, marks its once-each-turn use, and applies the
// chosen-colour Animate to itself until end of turn
// (kop.ManaRiderChosenAnimate; kernel mana_rider.go). A second activation
// this turn is refused.
func TestWickermawChosenAnimateRider(t *testing.T) {
	sc, wick := wickermawScen(t)
	sl := sc.slot(wick)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok || a.ManaRider != kop.ManaRiderChosenAnimate || a.Cost.Flags != kop.CostOncePerTurn || a.ManaColors != kop.ColAny || len(a.Prog) != 1 {
		t.Fatalf("precondition: ability %+v is not the chosen-colour rider", a)
	}
	if !sc.s.riderActivatable(sc.k, 0, sl, a) {
		t.Fatal("precondition: the source is not activatable")
	}
	if got := sc.s.colorsAt(sc.k, sl); got != 0 {
		t.Fatalf("precondition: the source already has colour %#x", got)
	}
	if !sc.s.riderGap(sc.k, 0, sl, int(kop.G)) {
		t.Fatal("the rider activation failed")
	}
	if got := sc.s.pl[0].pool; got[kop.C] != 0 || got[kop.G] != 1 {
		t.Fatalf("pool %v after the activation, want one green and the fee spent", got)
	}
	if got := sc.s.perms[sl].actUsed; got&1 == 0 {
		t.Fatal("the once-each-turn use was not spent")
	}
	if got := sc.s.colorsAt(sc.k, sl); got != kop.ColG {
		t.Fatalf("the source is colour %#x, want green", got)
	}
	if sc.s.riderActivatable(sc.k, 0, sl, a) {
		t.Fatal("the used source is still activatable")
	}
}
