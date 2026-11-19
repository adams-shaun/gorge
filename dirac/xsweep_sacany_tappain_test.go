package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestXSweepSacrificeAnyRepeatWithATapsPainWatcher: the sweep census's
// unrecord must take back the pain the ordinary unit was recorded with
// (cen.unitPain: abPain plus the tap's foreign damage, a Pain-class Taps
// watcher's), not abPain alone — an unrecord that misses silently no-ops
// (unrecordPotUnit finds no unit at that pain) and leaves the census at
// 1+n any-colour units where the per-payment bound is n, one extra unit the
// lethal guard prices at (life-1)/pain (CR 605.3b, 601.2g, 119.4). The
// offer window itself stays blind to the phantom (the real payment planner
// funds two activations either way), so the probe reads the sweep's
// potUnit record directly: beside a Pain-class Taps watcher, two
// sacrificeable creatures must price as exactly two any-colour units at
// cen.pain with the ordinary unit's pain-1 record taken back.
func TestXSweepSacrificeAnyRepeatWithATapsPainWatcher(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{} // a non-nil expression table makes prepareGen build the generated extension (vanilla alone has none)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	creature := filter(kop.Filter{Types: kop.TCreature})
	artifact := filter(kop.Filter{Types: kop.TArtifact})
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
	watch := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pain Watch", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTaps, TapPlan: kop.TapPlanPain, TapPainN: 1, Filter: artifact}}}})
	finale := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Finale", Types: kop.TSorcery,
		Cost: kop.ManaCost{X: 1, Pips: [kop.NumManaKinds]uint8{int(kop.ColB): 2}}, // {X}{B}{B}
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: toughLE, Min: 0, Max: 3, UpTo: true}},
			NTargets: 1,
			Prog:     []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})

	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		sc := newScenTableLib(t, &tbl, v.Plains, cfg)
		k := sc.k
		alt := sc.perm(altar, 0)
		sc.perm(bear, 0)
		sc.perm(bear, 0)
		sc.perm(watch, 0)
		card := sc.hand(finale, 0)
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
		if got := sc.s.tapsCensusPain(k, 0, sc.slot(alt)); got != 1 {
			t.Fatalf("gorgeLowering=%v: precondition: the Altar's census tap pain %d, want the Pain-class watcher's 1", cfg.GorgeLowering, got)
		}
		var pot avail
		var pu [maxPotUnits]potUnit
		d := payDesc{class: pdSpell, card: sc.s.objs[card].card, src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
		sc.s.availPotentialFor(k, 0, &pot, &d, true, &pu)
		if pu[0].mask != 0 && pu[0].n == 0 {
			t.Fatalf("gorgeLowering=%v: the sweep's census record is incomplete (the overflow marker): the Altar's repeats priced no exact view", cfg.GorgeLowering)
		}
		units, phantom := 0, 0
		for i := 0; i < maxPotUnits && pu[i].mask != 0; i++ {
			u := pu[i]
			if u.mask != uint8(kop.ColAny) {
				t.Fatalf("gorgeLowering=%v: unexpected census unit %d of mask %#x", cfg.GorgeLowering, i, u.mask)
			}
			if u.pain != 0 {
				phantom += int(u.n) // the ordinary unit the unrecord must take back
				continue
			}
			units += int(u.n)
		}
		if phantom != 0 || units != 2 {
			t.Fatalf("gorgeLowering=%v: two candidates fund two activations, but the census prices %d any-colour units at cen.pain and %d left over at the ordinary unit's pain (the watcher's tap damage): the unrecord must take back cen.unitPain", cfg.GorgeLowering, units, phantom)
		}
	}
}
