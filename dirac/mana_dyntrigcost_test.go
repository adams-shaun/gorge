package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dynPayTable builds a two-seat matchup over a pool table with a synthetic
// counter-scaled sacrifice mana source appended ("Add X mana of any one
// color, where X is the number of PETAL counters on this", Lotus Blossom's
// shape: dynamicManaCost's {T}, SacSelf with an AfManaDynamic amount), a
// fixed one-face variant ("Add one G for each PETAL counter on this", Elvish
// Archdruid's shape: the same cost with ManaColors ColG), a synthetic 1/1
// whose attack trigger carries an optional own-cost ask: OpIfPaid {6} (C 17 =
// the PayMana kind with an empty skip nibble, as
// mana_rider_trigcost_test.go's rider), then draw a card, and a twin whose
// ask is {2}{B}{B} (the pips in B, manaOf's encoding). The pool table's own
// cards stay out of the boards.
func dynPayTable(t *testing.T) (*kop.Table, CardID, CardID, CardID, CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := da[0]
	lotus := CardID(len(tbl.Cards))
	trig := lotus + 1
	fixed := trig + 1
	pipsTrig := fixed + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Petals", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostSacSelf},
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)},
			ManaColors: kop.ColAny,
		}}},
		kop.CardDef{Name: "Test Dyn Trigger Cost", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
				Prog: []kop.Instr{
					{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(6), B: kop.Const(0), C: 17},
					{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
				}}}},
		kop.CardDef{Name: "Test Fixed Petals", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostSacSelf},
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)},
			ManaColors: kop.ColG,
		}}},
		kop.CardDef{Name: "Test Dyn Trigger Cost Pips", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
				Prog: []kop.Instr{
					{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(2 << 6), C: 17},
					{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
				}}}},
	)
	return tbl, plains, lotus, trig, fixed, pipsTrig
}

// dynPayAsk fires the synthetic attack trigger and walks to the next choose
// decision: the payer's trigger-cost pay ask.
func dynPayAsk(t *testing.T, sc *scen, trig CardID) Decision {
	t.Helper()
	tr := sc.perm(trig, 0)
	sc.s.fireObj(sc.k, kop.TrigAttacks, sc.slot(tr), tr, 0, 0)
	var d Decision
	for i := 0; i < 12; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose {
			return d
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	t.Fatalf("no choose decision in 12 steps, last kind %d sub %d", d.Kind, d.Sub)
	return d
}

// dynProbe poses the trigger-cost gate's inputs: the resolution is a
// trigger's and the pool is empty, as gorge's ask reads it.
func dynProbe(t *testing.T, sc *scen) {
	t.Helper()
	for i := range sc.s.pl[0].pool {
		if sc.s.pl[0].pool[i] != 0 {
			t.Fatalf("precondition: pool colour %d is %d, want an empty pool", i, sc.s.pl[0].pool[i])
		}
	}
	sc.s.res.e.kind = seTrigger
}

// TestDynTriggerCostPay: a trigger's own optional mana cost is offered on the
// payer's POTENTIAL mana (rules respay.go resPayNeedsMana over potential.go),
// and gorge's potential walk prices a mana ability whose Amount$ it cannot
// statically price potentialUnbounded in every colour (potential.go
// addPotentialMana: an indeterminate Amount$ is never priced at zero) --
// Lotus Blossom's "Add X mana of any one color" with no petal counters on it.
// The kernel's offer gate (interp_ask.go trigCostPotentialPays) priced the
// census's current amount, zero petals being zero mana and no planner unit
// (gen.go varPC, mana_sacctr.go's census drop), so the ask was decline-only
// while gorge offered pay and decline (CR 118.3: the payer elects pay, the
// charge fails, the payer declines).
func TestDynTriggerCostPay(t *testing.T) {
	tbl, plains, lotus, trig, _, _ := dynPayTable(t)

	// live: beside a source that yields now, the gate opens and the ask offers
	// decline then pay
	sc := newScenTable(t, tbl, Config{NoElide: true})
	l := sc.perm(lotus, 0)
	sc.perm(plains, 0)
	if c := sc.s.cardAt(sc.slot(l)); sc.k.gen.flags[c]&gfManaVar == 0 || sc.k.gen.manaVar(c) < 0 {
		t.Fatal("precondition: the petal source is not the table's dynamic-amount source")
	}
	dynProbe(t, sc)
	c6 := manaOf(6, 0)
	if !sc.s.potentialDynOpen(sc.k, 0, &c6, nil) || !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the counter-less petal source")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(lotus, 0)
	scA.perm(plains, 0)
	if d := dynPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live ask %d options %v, want decline then pay: gorge's potential prices the dynamic amount unbounded", d.Sub, d.Options[:d.N])
	}

	// alone: no source yields now, gorge's sources flag is false, decline only
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	sc2.perm(lotus, 0)
	dynProbe(t, sc2)
	if sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {6} beside a counter-less petal source alone")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	sc2A.perm(lotus, 0)
	if d := dynPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("alone ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// tapped: gorge's walk skips the source, decline only
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	l3 := sc3.perm(lotus, 0)
	sc3.perm(plains, 0)
	sc3.s.perms[sc3.slot(l3)].flags |= pfTapped
	dynProbe(t, sc3)
	if sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {6} beside a tapped petal source")
	}
	sc3A := newScenTable(t, tbl, Config{NoElide: true})
	l3A := sc3A.perm(lotus, 0)
	sc3A.perm(plains, 0)
	sc3A.s.perms[sc3A.slot(l3A)].flags |= pfTapped
	if d := dynPayAsk(t, sc3A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// petals: the source itself yields two now, and the amount is still
	// indeterminate, so the gate opens with no other source on the board
	sc4 := newScenTable(t, tbl, Config{NoElide: true})
	l4 := sc4.perm(lotus, 0)
	sc4.s.perms[sc4.slot(l4)].setCtrN(sc4.s, int(kop.CtrPetal), 2)
	dynProbe(t, sc4)
	if !sc4.s.trigCostPotentialPays(sc4.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside a two-petal petal source")
	}
	sc4A := newScenTable(t, tbl, Config{NoElide: true})
	l4A := sc4A.perm(lotus, 0)
	sc4A.s.perms[sc4A.slot(l4A)].setCtrN(sc4A.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, sc4A, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("petals ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}

// TestDynTriggerCostPayFixedFace: the potentialUnbounded fold is per produced
// face, not per colour (gorge rules potential.go addIndeterminatePotentialMana,
// CR 107.4a: a fixed Produced$ names the only colours the ability makes). A
// fixed-face dynamic-amount source ("Add one G for each PETAL counter on
// this", Elvish Archdruid's shape) beside a trigger's own cost whose pips lie
// outside its faces is decline-only on both engines -- gorge's offer gate
// still requires the potential pool to cover the cost's coloured pips
// (respay.go resPayNeedsMana over pay.CostPayablePool, {2}{B}{B} beside G
// faces: B = 0) -- while the kernel's gate opened on any admitted
// dynamic-amount source, whatever its production makes.
func TestDynTriggerCostPayFixedFace(t *testing.T) {
	tbl, _, _, trig, fixed, pipsTrig := dynPayTable(t)

	// pips outside the faces: the gate stays closed, decline only (gorge's
	// CostPayablePool refuses the B pips the G faces cannot fund)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	l := sc.perm(fixed, 0)
	sc.s.perms[sc.slot(l)].setCtrN(sc.s, int(kop.CtrPetal), 2)
	if c := sc.s.cardAt(sc.slot(l)); sc.k.gen.flags[c]&gfManaVar == 0 || sc.k.gen.manaVar(c) < 0 {
		t.Fatal("precondition: the fixed petal source is not the table's dynamic-amount source")
	}
	dynProbe(t, sc)
	pipsCost := manaOf(2, 2<<6)
	if sc.s.potentialDynOpen(sc.k, 0, &pipsCost, nil) || sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 2, 2<<6, 0) {
		t.Fatal("the gate offered the {2}{B}{B} beside a fixed-face G source: gorge's over-bound is unbounded in the faces only")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	lA := scA.perm(fixed, 0)
	scA.s.perms[scA.slot(lA)].setCtrN(scA.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, scA, pipsTrig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("fixed-face pip ask %d options %v, want decline only: gorge's pool covers no B pip from G faces", d.Sub, d.Options[:d.N])
	}

	// the generic alone: one face funds it, the gate opens and the ask offers
	// decline then pay (gorge's unbounded G funds any amount of generic)
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	l2 := sc2.perm(fixed, 0)
	sc2.s.perms[sc2.slot(l2)].setCtrN(sc2.s, int(kop.CtrPetal), 2)
	dynProbe(t, sc2)
	c6 := manaOf(6, 0)
	if !sc2.s.potentialDynOpen(sc2.k, 0, &c6, nil) || !sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate refused the {6} beside a fixed-face G source: gorge's over-bound funds the generic from one face")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	l2A := sc2A.perm(fixed, 0)
	sc2A.s.perms[sc2A.slot(l2A)].setCtrN(sc2A.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("fixed-face generic ask %d options %v, want decline then pay: gorge's unbounded G funds the generic", d.Sub, d.Options[:d.N])
	}
}
