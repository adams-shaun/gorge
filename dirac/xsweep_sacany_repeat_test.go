package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestXSweepCensusCountsASacrificeAnyRepeat: the X-bound-target sweep's
// census (xBoundPayable's potUnitsAvail record) must keep a live
// sacrifice-any mana source's repeats — Phyrexian Altar's "Sacrifice a
// creature: Add one mana of any color" (no {T}) activates once per
// candidate in the same payment, so two sacrificeable creatures fund
// {B}{B}, and gorge's planner finds the Altar×2 prefix and offers the
// {X}-bound-target cast at the announced X of zero with no targets (CR
// 605.3b, 601.2g). The sweep's record priced the Altar once, the cast fell
// out of every walk, and the kernel withheld it (tail-68-c002 pair 5).
// With one candidate the one-unit record is exact and the sweep still
// withholds a play that one activation cannot fund, as gorge's planner does.
func TestXSweepCensusCountsASacrificeAnyRepeat(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{} // a non-nil expression table makes prepareGen build the generated extension (vanilla alone has none)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	creature := filter(kop.Filter{Types: kop.TCreature})
	toughLE := filter(kop.Filter{Types: kop.TCreature, Pred: kop.PredToughnessLEV, PredV: kop.ValueRef{K: kop.VX}})
	altar := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Altar", Types: kop.TArtifact,
		Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostSac, N: 1, Filter: creature},
			ManaColors: kop.ColAny, ManaN: 1,
		}}})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	finale := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Finale", Types: kop.TSorcery,
		Cost: kop.ManaCost{X: 1, Pips: [kop.NumManaKinds]uint8{int(kop.ColB): 2}}, // {X}{B}{B}
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: toughLE, Min: 0, Max: 3, UpTo: true}},
			NTargets: 1,
			Prog:     []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})

	board := func(bears int, cfg Config) (*scen, Obj, Obj) {
		t.Helper()
		sc := newScenTableLib(t, &tbl, v.Plains, cfg)
		alt := sc.perm(altar, 0)
		for i := 0; i < bears; i++ {
			sc.perm(bear, 0)
		}
		card := sc.hand(finale, 0)
		return sc, alt, card
	}
	castOf := func(sc *scen, o Obj) bool {
		t.Helper()
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				return true
			}
		}
		return false
	}
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		sc, alt, card := board(2, cfg)
		k := sc.k
		ab := k.tapXAbOf(sc.s.cardAt(sc.slot(alt)))
		if ab == nil || !kop.SacAnyManaAb(ab) {
			t.Fatalf("gorgeLowering=%v: precondition: the Altar is not the table's sacrifice-any mana ability", cfg.GorgeLowering)
		}
		if !sc.s.scriptTapX(k, 0).has(sc.slot(alt)) {
			t.Fatalf("gorgeLowering=%v: precondition: the live Altar is not a modelled tap-another gap", cfg.GorgeLowering)
		}
		var cs [MaxPerm]Slot
		if n := sc.s.sacXCands(k, 0, sc.slot(alt), ab, &cs); n != 2 {
			t.Fatalf("gorgeLowering=%v: precondition: sacrifice candidates %d, want 2", cfg.GorgeLowering, n)
		}
		if !castOf(sc, card) {
			t.Fatalf("gorgeLowering=%v: the two-creature Altar funds {B}{B} in the same payment, but the {X}-bound-target cast is not offered", cfg.GorgeLowering)
		}
		sc1, _, card1 := board(1, cfg)
		if castOf(sc1, card1) {
			t.Fatalf("gorgeLowering=%v: one candidate funds one activation, and the cast at the announced X of zero still needs {B}{B}: it must be withheld", cfg.GorgeLowering)
		}
	}
}
