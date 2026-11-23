package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// feeOrderTable is vanilla's table with the repro pair's cards appended as
// inline fixtures: a Mountain (vanilla's, twice), a {T}: B or R choice dual
// (Hell's Kitchen), a {T}: W or B one (Fisk Tower), a land with {T}: {C} and
// {T}: B or R (Dark Fortress's two abilities, the gate on the second left off:
// the pair's board holds the basic Mountain, so it is live there) and a Baxter
// Building: {T}: Add {C}, and {4},{T}: four mana in any combination of colours
// -- the multi-mana fee filter. The held spell is Ant-Man's Army's {2}{G}. The
// corpus's synthetic pools are evidence tables, which only the lockstep oracle
// and kgen may read (internal/archtest TestOnlyTheOracleReadsRawTables), so
// the board is built here. The board is the fixture's own, wider than the
// pair's: the pair's plain run no longer exhausts its node budget at the
// current base, so the copies of the duals and a second filter widen the
// funding tree until it does.
func feeOrderTable() (*kop.Table, []CardID, CardID, CardID) {
	tbl := v.NewTable()
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	choice := func(name string, col kop.ColorMask) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TLand,
			Abilities: []kop.Ability{{Kind: kop.AbMana, AFlags: kop.AfManaChoice,
				Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: col, ManaN: 1}}}
	}
	mk := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, choice("Test Kitchen", kop.ColB|kop.ColR))
	ft := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, choice("Test Tower", kop.ColW|kop.ColB))
	df := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Fortress", Types: kop.TLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, AFlags: kop.AfManaChoice, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: kop.ColB | kop.ColR, ManaN: 1}}})
	baxter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Baxter", Types: kop.TLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, AFlags: kop.AfManaChoice,
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 4}, Flags: kop.CostTap},
				ManaColors: kop.ColAny, ManaN: 4}}})
	sp := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Army", Cost: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.G: 1}},
		Types: kop.TCreature, Colors: kop.ColG, Power: 1, Toughness: 1})
	return tbl, []CardID{mk, ft, df}, baxter, sp
}

// TestFilterFeeOrderFindsBaxterWithDualLands: the pair's board -- Mountains,
// two choice duals, a colourless land and the {4},{T} four-mana
// any-combination filter -- funds a {2}{G} cast. No source makes G but the
// filter, whose {4} fee the lands' own mana pays (CR 601.2g). The plain run
// spends its node budget on the filter's fee branches before the fee is
// funded; the fee-order run (scriptSearch.feeOrder, mana_filterfeeorder.go)
// walks the funding first, so the cast is offered. It mirrors lockstep pair
// combination_filter_fee_paid_by_dual_lands (pool std-msh).
func TestFilterFeeOrderFindsBaxterWithDualLands(t *testing.T) {
	tbl, lands, baxter, sp := feeOrderTable()
	// PRECONDITION: no source makes G but the filter, and the lands' own mana
	// (all untapped) funds the filter's {4} fee.
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	sc.perm(lands[0], 0) // Test Kitchen, the pair's Hell's Kitchen
	sc.perm(lands[1], 0) // Test Tower, the pair's Fisk Tower
	sc.perm(baxter, 0)
	sc.perm(lands[2], 0) // Test Fortress, the pair's Dark Fortress
	// The widening: a second copy of each dual land and a second filter. They
	// stretch the plain run's funding tree past potentialScriptNodes (160) before
	// any fee is funded, while the fee-order run still finishes inside it (with
	// the fallback off the cast is not offered; with 4 filters neither run
	// finishes).
	sc.perm(lands[0], 0)
	sc.perm(lands[1], 0)
	sc.perm(lands[2], 0)
	sc.perm(baxter, 0)
	held := sc.hand(sp, 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(held) {
			return
		}
	}
	t.Fatalf("the {2}{G} spell is not offered: Baxter's {4},{T} makes G from the lands' mana (opts %v)", d.Options[:d.N])
}
