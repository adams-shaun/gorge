package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Taps-deferred script gap is only the plain one-mana tap of any colour
// (Firdoch Core) or of one fixed colour (Llanowar Elves). A production it does not model keeps its ordinary census
// unit in availFor (the skip is keyed on the script mask); a restricted one
// would float plain pool mana; and the anyR arm of tapGapChoice taps and fires
// the Taps watchers exactly once. Each test mutates a copied tapsdefer table,
// so the shared pool is untouched.

// tapsDeferShapeTable is the tapsdefer pool's table with the deck's cards and
// one mutation applied to a copied table (no ability slice is shared with the
// pool).
func tapsDeferShapeTable(t *testing.T, mutate func(tbl *kop.Table, id map[string]CardID)) (*kop.Table, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Judge of Currents", Count: 1}, {Name: "Firdoch Core", Count: 1},
		{Name: "Birds of Paradise", Count: 1}, {Name: "Plains", Count: 19},
	}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tc := *tbl
	tc.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	mutate(&tc, id)
	return &tc, id
}

// rewriteMana replaces card c's abilities with a copy and applies f to its
// first mana ability.
func rewriteMana(tbl *kop.Table, c CardID, f func(a *kop.Ability)) {
	cd := &tbl.Cards[c]
	cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
	for i := range cd.Abilities {
		if cd.Abilities[i].Kind == kop.AbMana {
			f(&cd.Abilities[i])
			return
		}
	}
	panic("no mana ability")
}

// TestTapsDeferUnmodelledProductionKeepsItsCensusUnit: a two-mana deferred
// source (the Core rewritten to "{T}: Add {G}{G}") is no script gap, so
// availFor must still count it; dropping every deferred source from the
// walk made a play only it funds unofferable. (A one-mana fixed-colour tap
// is a modelled gap: mana_tapsdefer_mono_test.go.)
func TestTapsDeferUnmodelledProductionKeepsItsCensusUnit(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) { a.ManaColors, a.ManaN = kop.ColG, 2 })
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	core := sc.perm(id["Firdoch Core"], 0)
	sc.perm(id["Judge of Currents"], 0)
	if !sc.s.tapsDeferred(sc.k, 0, sc.slot(core)) {
		t.Fatal("precondition: the two-mana Core is not a deferred source under Judge of Currents")
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(core)); pc.manaN != 2 {
		t.Fatalf("precondition: the Core's production is %d mana, want 2", pc.manaN)
	}
	if sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
		t.Fatal("a two-mana production is in the script gap mask: tapDeferFloat floats one mana of it")
	}
	var a avail
	sc.s.g.availX = availGaps
	sc.s.availFor(sc.k, 0, &a)
	sc.s.g.availX = 0
	if a.total == 0 {
		t.Fatal("availFor counts no mana: the deferred fixed-colour source was dropped from every walk")
	}
}

// TestTapsDeferRestrictedProductionIsNoScriptGap: a restricted production
// must stay out of the mask; tapDeferFloat adds plain pool mana and does not
// track the restriction. A restricted first mana ability already clears the
// planner view's manaAb (gen.go), so the guards cover the card flag and the
// ability's own ManaRestrict (the granted-source path keeps manaAb).
func TestTapsDeferRestrictedProductionIsNoScriptGap(t *testing.T) {
	t.Run("a restricted card flag", func(t *testing.T) {
		tbl, id := tapsDeferShapeTable(t, func(*kop.Table, map[string]CardID) {})
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		core := sc.perm(id["Firdoch Core"], 0)
		sc.perm(id["Judge of Currents"], 0)
		if !sc.s.tapsDeferred(sc.k, 0, sc.slot(core)) {
			t.Fatal("precondition: the Core is not a deferred source under Judge of Currents")
		}
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("precondition: the plain Core is not in the script gap mask")
		}
		sc.k.gen.flags[sc.s.cardAt(sc.slot(core))] |= gfRestr
		if sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("a restricted source is in the script gap mask: its tap would float plain pool mana")
		}
	})
	t.Run("the ability's own restriction", func(t *testing.T) {
		tbl, id := tapsDeferShapeTable(t, func(*kop.Table, map[string]CardID) {})
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		core := sc.perm(id["Firdoch Core"], 0)
		sc.perm(id["Judge of Currents"], 0)
		if !sc.s.tapsDeferred(sc.k, 0, sc.slot(core)) {
			t.Fatal("precondition: the Core is not a deferred source under Judge of Currents")
		}
		pc := sc.s.manaPCGen(sc.k, sc.slot(core))
		if pc.manaAb < 0 || pc.manaCol != kop.ColAny || pc.manaN != 1 {
			t.Fatalf("precondition: Core view ab %d col %#x n %d: want the any-colour shape", pc.manaAb, pc.manaCol, pc.manaN)
		}
		// Mutate after preparation, so the planner view keeps manaAb: the
		// shape guard's own ManaRestrict test is what must reject it.
		sc.k.t.Cards[sc.s.cardAt(sc.slot(core))].Abilities[pc.manaAb].ManaRestrict = 1
		if sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("a restricted ability is in the script gap mask: its tap would float plain pool mana")
		}
	})
}

// TestScriptAnyRGapFiresItsTapsWatcherOnce: the anyR arm of tapGapChoice
// taps once and fires the Taps listeners once. The Birds become the wide
// restricted any-colour gap; Judge becomes a Pain-class Taps watcher (it
// still fires on the tap, but does not defer the source), so it is not the
// tapsDef arm.
func TestScriptAnyRGapFiresItsTapsWatcherOnce(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) {
			a.ManaColors = kop.ColAny
			a.ManaN = 2
			a.ManaRestrict = 1
			a.AFlags |= kop.AfManaChoice
		})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append([]kop.Ability(nil), j.Abilities...)
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature})
		j.Abilities[0].Trig.Filter = kop.FilterID(len(tbl.Filters) - 1)
		j.Abilities[0].Trig.TapPlan = kop.TapPlanPain
		j.Abilities[0].Trig.TapPainN = 1
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	birds := sc.perm(id["Birds of Paradise"], 0)
	sc.perm(id["Judge of Currents"], 0)
	anyR := sc.s.scriptAnyR(sc.k, 0)
	if !anyR.has(sc.slot(birds)) {
		t.Fatal("precondition: the Birds are not the anyR gap")
	}
	if sc.s.tapsDeferred(sc.k, 0, sc.slot(birds)) {
		t.Fatal("precondition: the Birds are deferred, so the tapsDef arm, not anyR, would run")
	}
	if sc.s.npend != 0 {
		t.Fatalf("precondition: %d pending triggers before the tap", sc.s.npend)
	}
	var spill [kop.NumManaKinds]uint8
	sc.s.tapGapChoice(sc.k, 0, sc.slot(birds), slotMask{}, anyR, slotMask{}, 0, &spill)
	if got := sc.s.npend; got != 1 {
		t.Fatalf("%d Taps triggers queued by one scripted anyR tap, want 1", got)
	}
}
