package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// dynGapTable appends the non-varPC dynamic-amount mana sources to dynPayTable:
// a paid filter ("{2}, {T}: Add one B for each PETAL counter on this", Cabal
// Coffers' cost shape), an each-colour gap ("{T}: For each colour among
// permanents you control, add one mana of that colour", Bloom Tender's shape)
// and a restricted dynamic source ("{T}: Add one B for each PETAL counter on
// this. Spend this mana only to activate abilities.", Redshift, Rocketeer
// Chief's shape: AfManaNonSpell admits a trigger's own cost).
func dynGapTable(t *testing.T) (*kop.Table, CardID, CardID, CardID, CardID, CardID) {
	t.Helper()
	tbl, plains, _, trig, _, _ := dynPayTable(t)
	filter := CardID(len(tbl.Cards))
	each := filter + 1
	restr := each + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Paid Filter Petals", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 2}},
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)},
			ManaColors: kop.ColB,
		}}},
		kop.CardDef{Name: "Test Each Colour Gap", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap},
			AFlags:     kop.AfManaEachColor,
			ManaN:      1,
			ManaColors: kop.ColAny,
		}}},
		kop.CardDef{Name: "Test Restricted Petals", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:         kop.AbMana,
			Cost:         kop.Cost{Flags: kop.CostTap},
			AFlags:       kop.AfManaDynamic | kop.AfManaNonSpell,
			ManaV:        kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)},
			ManaColors:   kop.ColB,
			ManaRestrict: 1,
		}}},
	)
	return tbl, plains, trig, filter, each, restr
}

// dynShapePreconditions checks the shape is one the census leaves out of the
// dynamic-amount planner view (no gfManaVar), so the gate's fold is the only
// unbounded claim.
func dynShapePreconditions(t *testing.T, sc *scen, l Obj, shape string) {
	t.Helper()
	c := sc.s.cardAt(sc.slot(l))
	if sc.k.gen.flags[c]&gfManaVar != 0 {
		t.Fatalf("precondition: the %s source is a varPC source, not the shape under test", shape)
	}
}

// TestDynTriggerCostPayRestrictedExtension is TestDynTriggerCostPay for the
// restricted dynamic-amount shape: a mana ability whose Amount$ is not a
// literal and whose RestrictValid$ admits a trigger's own cost
// (AfManaNonSpell, Redshift's "spend this mana only to activate abilities").
// gorge's potential walk prices an indeterminate Amount$ potentialUnbounded in
// the produced faces whether or not the ability is the card's planner mana
// ability (rules potential.go addIndeterminatePotentialMana), so beside a live
// source the trigger's "may pay" is offered; the census drops the source from
// its planner view (gen.go clears manaAb for a dynamic or restricted ability),
// and its strict read (potentialAdmits' sacCtr zero exclusion) keeps a source
// that makes nothing now out of the sources flag (gorge windowSourceYields).
func TestDynTriggerCostPayRestrictedExtension(t *testing.T) {
	tbl, plains, trig, _, _, restr := dynGapTable(t)
	c6 := manaOf(6, 0)

	// live: a Plains beside the counter-less restricted source. The
	// potential walk admits the restricted ability (its {T} cost is payable)
	// and prices the indeterminate amount unbounded in B, so the gate opens.
	sc := newScenTable(t, tbl, Config{NoElide: true})
	lr := sc.perm(restr, 0)
	sc.perm(plains, 0)
	dynShapePreconditions(t, sc, lr, "restricted")
	dynProbe(t, sc)
	if !sc.s.potentialDynOpen(sc.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) {
		t.Fatal("potentialDynOpen refused the restricted dynamic source beside a live source")
	}
	if !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the restricted dynamic source and a live source")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(restr, 0)
	scA.perm(plains, 0)
	if d := dynPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}

	// alone: the only source makes nothing now, so gorge's sources flag is
	// false and the ask is decline only (a pay offer would be the over-bound
	// gate riding a source windowSourceYields refuses).
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	sc2.perm(restr, 0)
	dynProbe(t, sc2)
	if sc2.s.potentialDynOpen(sc2.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside a counter-less restricted source alone: it makes nothing now")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	sc2A.perm(restr, 0)
	if d := dynPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("alone ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// tapped: the walk skips the source, and the Plains alone funds no {6}.
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	l3 := sc3.perm(restr, 0)
	sc3.perm(plains, 0)
	sc3.s.perms[sc3.slot(l3)].flags |= pfTapped
	dynProbe(t, sc3)
	if sc3.s.potentialDynOpen(sc3.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside a tapped restricted source")
	}
	sc3A := newScenTable(t, tbl, Config{NoElide: true})
	l3A := sc3A.perm(restr, 0)
	sc3A.perm(plains, 0)
	sc3A.s.perms[sc3A.slot(l3A)].flags |= pfTapped
	if d := dynPayAsk(t, sc3A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// counters: the source itself yields two now, so the sources flag holds
	// and the indeterminate amount stays unbounded.
	sc4 := newScenTable(t, tbl, Config{NoElide: true})
	l4 := sc4.perm(restr, 0)
	sc4.s.perms[sc4.slot(l4)].setCtrN(sc4.s, int(kop.CtrPetal), 2)
	dynProbe(t, sc4)
	if !sc4.s.potentialDynOpen(sc4.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		!sc4.s.trigCostPotentialPays(sc4.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate refused the {6} beside a two-petal restricted source, which yields now")
	}
	sc4A := newScenTable(t, tbl, Config{NoElide: true})
	l4A := sc4A.perm(restr, 0)
	sc4A.s.perms[sc4A.slot(l4A)].setCtrN(sc4A.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, sc4A, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("petals ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}

// TestDynTriggerCostPayFilterExtension is the paid-filter shape: a {2}, {T}
// dynamic-amount source (Cabal Coffers' cost shape). The potential walk admits
// the ability once the other sources cover the {2} activation cost, and then
// prices the indeterminate amount unbounded in B; a board with the fee
// uncovered stays decline-only.
func TestDynTriggerCostPayFilterExtension(t *testing.T) {
	tbl, plains, trig, filter, _, _ := dynGapTable(t)
	c6 := manaOf(6, 0)

	// live: two Plains fund the {2}, the dynamic B is unbounded.
	sc := newScenTable(t, tbl, Config{NoElide: true})
	lf := sc.perm(filter, 0)
	sc.perm(plains, 0)
	sc.perm(plains, 0)
	dynShapePreconditions(t, sc, lf, "paid filter")
	dynProbe(t, sc)
	if !sc.s.potentialDynOpen(sc.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) {
		t.Fatal("potentialDynOpen refused the paid filter whose fee two Plains fund")
	}
	if !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the fee-fundable paid filter")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(filter, 0)
	scA.perm(plains, 0)
	scA.perm(plains, 0)
	if d := dynPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}

	// alone: the {2} is uncovered, so the walk never admits the ability.
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	sc2.perm(filter, 0)
	dynProbe(t, sc2)
	if sc2.s.potentialDynOpen(sc2.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside a paid filter whose fee is uncovered")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	sc2A.perm(filter, 0)
	if d := dynPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("alone ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// tapped: the walk skips the source even with the fee funded.
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	l3 := sc3.perm(filter, 0)
	sc3.perm(plains, 0)
	sc3.perm(plains, 0)
	sc3.s.perms[sc3.slot(l3)].flags |= pfTapped
	dynProbe(t, sc3)
	if sc3.s.potentialDynOpen(sc3.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside a tapped paid filter")
	}
	sc3A := newScenTable(t, tbl, Config{NoElide: true})
	l3A := sc3A.perm(filter, 0)
	sc3A.perm(plains, 0)
	sc3A.perm(plains, 0)
	sc3A.s.perms[sc3A.slot(l3A)].flags |= pfTapped
	if d := dynPayAsk(t, sc3A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// counters: the source yields two now, the fee is funded, the amount is
	// still indeterminate.
	sc4 := newScenTable(t, tbl, Config{NoElide: true})
	l4 := sc4.perm(filter, 0)
	sc4.perm(plains, 0)
	sc4.perm(plains, 0)
	sc4.s.perms[sc4.slot(l4)].setCtrN(sc4.s, int(kop.CtrPetal), 2)
	dynProbe(t, sc4)
	if !sc4.s.potentialDynOpen(sc4.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		!sc4.s.trigCostPotentialPays(sc4.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate refused the {6} beside a two-petal paid filter with the fee funded")
	}
	sc4A := newScenTable(t, tbl, Config{NoElide: true})
	l4A := sc4A.perm(filter, 0)
	sc4A.perm(plains, 0)
	sc4A.perm(plains, 0)
	sc4A.s.perms[sc4A.slot(l4A)].setCtrN(sc4A.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, sc4A, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("petals ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}

// TestDynTriggerCostPayEachColourExtension is the each-colour census gap
// (Bloom Tender's shape): the ability's production is not a fixed face set, so
// gorge's payment fold prices it open in every colour even at a literal
// Amount$ (potential.go producedOpen over addPotentialMana). The kernel's
// census reads the colours among the board's permanents, which is one colour
// on a board of one green creature, so the unbounded claim has to come from
// the dynamic-shape fold.
func TestDynTriggerCostPayEachColourExtension(t *testing.T) {
	tbl, _, trig, _, each, _ := dynGapTable(t)
	c6 := manaOf(6, 0)

	sc := newScenTable(t, tbl, Config{NoElide: true})
	le := sc.perm(each, 0)
	dynShapePreconditions(t, sc, le, "each-colour")
	dynProbe(t, sc)
	if !sc.s.potentialDynOpen(sc.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) {
		t.Fatal("potentialDynOpen refused the each-colour gap: gorge prices its open production unbounded")
	}
	if !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the each-colour gap")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(each, 0)
	if d := dynPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("each-colour ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}
