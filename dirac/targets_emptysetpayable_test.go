package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// emptySetTable is an FDN Plains matchup with a spell of "up to two target
// creatures" costing {4}{U} and a Test Reducer creature whose static makes
// spells you cast that target a creature you control cost {3} less (Killian,
// Ink Duelist's battlefield ValidTarget$ shape). The reduction is on the
// battlefield card, so only the cost-composition read can see the cast is
// target-priced: the card-local spellTgtDep is false for the spell.
func emptySetTable(t *testing.T) (tbl *kop.Table, spell, reducer CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{},
		kop.Filter{Types: kop.TCreature},
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	nf := len(tbl.Filters)
	fAny, fCreature, fMine := kop.FilterID(nf-3), kop.FilterID(nf-2), kop.FilterID(nf-1)
	spell = CardID(len(tbl.Cards))
	reducer = spell + 1
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Pair Spell", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 4, Pips: [kop.NumManaKinds]uint8{kop.U: 1}},
			Abilities: []kop.Ability{{Kind: kop.AbSpell, NTargets: 1,
				Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fCreature, Min: 0, Max: 2}},
				Prog:    []kop.Instr{{Op: kop.OpTap, Obj: kop.RefTarget0}}}}},
		kop.CardDef{Name: "Test Reducer", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceCost,
				Filter: fAny, Filter2: fMine, A: 3, StFlags: kop.StTargetsFilter2}}}})
	if cardTgtDepCost(&tbl.Cards[spell]) {
		t.Fatal("precondition: the spell carries no reduction itself")
	}
	return tbl, spell, reducer
}

// TestTargetAskWithholdsUnpayableEmptySetBattlefieldReduction: a battlefield
// target-dependent reduction prices the cast per target (CR 601.2c before
// 601.2f), so the "up to" empty answer is withheld when it leaves the printed
// cost unpayable, exactly as gorge raises that ask's minimum
// (targetAskWithholdsEmptyTargetSet reads every cost static).
func TestTargetAskWithholdsUnpayableEmptySetBattlefieldReduction(t *testing.T) {
	tbl, spellID, redID := emptySetTable(t)
	run := func(t *testing.T, pool [kop.NumManaKinds]uint8) Decision {
		sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
		sc.perm(redID, 0)
		sc.perm(redID, 1)
		spell := sc.hand(spellID, 0)
		sc.s.pl[0].pool = pool
		sc.s.ctl = ctlPriority
		d := sc.s.Step()
		offered := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(spell)) {
				sc.s.Apply(a)
				offered = true
				break
			}
		}
		if !offered {
			t.Fatalf("precondition: the reduced cast is offered; options %v", d.Options[:d.N])
		}
		for step := 0; step < 20; step++ {
			d = sc.s.Step()
			if d.Kind == DkTarget {
				return d
			}
			if d.N == 0 {
				break
			}
			sc.s.Apply(d.Options[0])
		}
		t.Fatalf("no target decision; last kind %d", d.Kind)
		return d
	}
	hasDone := func(d Decision) bool {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkDone {
				return true
			}
		}
		return false
	}
	t.Run("reduced price only", func(t *testing.T) {
		d := run(t, [kop.NumManaKinds]uint8{kop.U: 2})
		if d.N < 1 {
			t.Fatalf("precondition: the ask offers candidates: %v", d.Options[:d.N])
		}
		if hasDone(d) {
			t.Fatalf("Done offered with {U}{U} floating: the empty set leaves {4}{U} unpayable; options %v", d.Options[:d.N])
		}
	})
	t.Run("full price payable", func(t *testing.T) {
		d := run(t, [kop.NumManaKinds]uint8{kop.U: 1, kop.C: 4})
		if d.N < 1 || !hasDone(d) {
			t.Fatalf("Done withheld though the printed {4}{U} is payable: %v", d.Options[:d.N])
		}
	})
}
