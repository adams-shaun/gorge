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
			Kind:   kop.AbMana,
			Cost:   kop.Cost{Flags: kop.CostTap},
			AFlags: kop.AfManaEachColor,
			ManaN:  1,
			// the real lowering leaves ManaColors 0 (kgen build_ab.go),
			// so the census keeps no planner unit for the gap
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
// census reads the colours among the board's permanents (zero on a board of
// colourless permanents), so the unbounded claim has to come from the
// dynamic-shape fold. The source is the real shape: an AfManaEachColor ability
// with ManaColors 0 and no planner manaAb (kgen build_ab.go), so the live arm
// under test is the each-colour arm of anyLiveSource, not a printed planner
// unit.
func TestDynTriggerCostPayEachColourExtension(t *testing.T) {
	tbl, plains, trig, _, each, _ := dynGapTable(t)
	green := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Green Bystander", Types: kop.TCreature, Colors: kop.ColG, Power: 1, Toughness: 1})
	c6 := manaOf(6, 0)
	ask := func(sc *scen) Decision { return dynPayAsk(t, sc, trig) }

	// live: one white permanent makes the each-colour count nonzero, and the
	// gap's open production covers the {6} unbounded.
	sc := newScenTable(t, tbl, Config{NoElide: true})
	le := sc.perm(each, 0)
	sc.perm(plains, 0)
	dynShapePreconditions(t, sc, le, "each-colour")
	if c := sc.s.cardAt(sc.slot(le)); sc.k.cards[c].manaAb >= 0 || sc.k.gen.eachCol(c) < 0 {
		t.Fatalf("precondition: the each-colour source is not a census gap (cardManaAb %d)", sc.k.cards[c].manaAb)
	}
	dynProbe(t, sc)
	if !sc.s.potentialDynOpen(sc.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) {
		t.Fatal("potentialDynOpen refused the each-colour gap: gorge prices its open production unbounded")
	}
	if !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the each-colour gap")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(each, 0)
	scA.perm(plains, 0)
	if d := ask(scA); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("each-colour live ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}

	// alone: no permanent has a colour, so the each-colour count is zero and
	// gorge's sources flag refuses the source that makes nothing now.
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	sc2.perm(each, 0)
	dynProbe(t, sc2)
	if sc2.s.potentialDynOpen(sc2.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside the each-colour gap with a zero count")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	sc2A.perm(each, 0)
	if d := ask(sc2A); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("each-colour alone ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// tapped: the walk skips the each-colour gap, and the one white source
	// alone cannot fund the {6}.
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	l3 := sc3.perm(each, 0)
	sc3.perm(plains, 0)
	sc3.s.perms[sc3.slot(l3)].flags |= pfTapped
	dynProbe(t, sc3)
	if sc3.s.potentialDynOpen(sc3.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside a tapped each-colour gap")
	}
	sc3A := newScenTable(t, tbl, Config{NoElide: true})
	l3A := sc3A.perm(each, 0)
	sc3A.perm(plains, 0)
	sc3A.s.perms[sc3A.slot(l3A)].flags |= pfTapped
	if d := ask(sc3A); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("each-colour tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// count: a green permanent makes the count nonzero, so the gap itself is
	// the board's only source and still opens the gate (the each-colour
	// analogue of a dynamic source holding counters).
	sc4 := newScenTable(t, tbl, Config{NoElide: true})
	sc4.perm(each, 0)
	sc4.perm(green, 0)
	dynProbe(t, sc4)
	if !sc4.s.potentialDynOpen(sc4.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		!sc4.s.trigCostPotentialPays(sc4.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate refused the {6} beside an each-colour gap whose count is nonzero")
	}
	sc4A := newScenTable(t, tbl, Config{NoElide: true})
	sc4A.perm(each, 0)
	sc4A.perm(green, 0)
	if d := ask(sc4A); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("each-colour count ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}
