package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dynPayTable builds a two-seat matchup holding Lotus Blossom ("Add X mana of
// any one color, where X is the number of petal counters on Lotus Blossom") and
// Island, plus a synthetic 1/1 whose attack trigger carries an optional
// own-cost ask: OpIfPaid {6} (C 17 = the PayMana kind with an empty skip
// nibble, as mana_rider_trigcost_test.go's rider), then draw a card.
func dynPayTable(t *testing.T) (*kop.Table, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-74")
	deck := []cardset.Entry{{Name: "Lotus Blossom", Count: 4}, {Name: "Island", Count: 8}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	trig := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Dyn Trigger Cost", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(6), B: kop.Const(0), C: 17},
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
			}}},
	})
	return tbl, id, trig
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

// dynProbePoses the trigger-cost gate's inputs on a state whose payer's
// permanents were just built: the resolution is a trigger's and the pool is
// empty, as gorge's ask reads it.
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
	tbl, id, trig := dynPayTable(t)
	lotus := id["Lotus Blossom"]
	isl := id["Island"]

	// live: beside a source that yields now, the gate opens and the ask offers
	// decline then pay
	sc := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	l := sc.perm(lotus, 0)
	sc.perm(isl, 0)
	if c := sc.s.cardAt(sc.slot(l)); sc.k.gen.flags[c]&gfManaVar == 0 || sc.k.gen.manaVar(c) < 0 {
		t.Fatal("precondition: Lotus Blossom is not the table's dynamic-amount source")
	}
	dynProbe(t, sc)
	if !sc.s.potentialDynOpen(sc.k, 0, nil) || !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside the petal-less Lotus Blossom")
	}
	scA := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	scA.perm(lotus, 0)
	scA.perm(isl, 0)
	if d := dynPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live ask %d options %v, want decline then pay: gorge's potential prices the dynamic amount unbounded", d.Sub, d.Options[:d.N])
	}

	// alone: no source yields now, gorge's sources flag is false, decline only
	sc2 := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	sc2.perm(lotus, 0)
	dynProbe(t, sc2)
	if sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {6} beside a petal-less Lotus Blossom alone")
	}
	sc2A := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	sc2A.perm(lotus, 0)
	if d := dynPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("alone ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// tapped: gorge's walk skips the source, decline only
	sc3 := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	l3 := sc3.perm(lotus, 0)
	sc3.perm(isl, 0)
	sc3.s.perms[sc3.slot(l3)].flags |= pfTapped
	dynProbe(t, sc3)
	if sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {6} beside a tapped Lotus Blossom")
	}
	sc3A := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	l3A := sc3A.perm(lotus, 0)
	sc3A.perm(isl, 0)
	sc3A.s.perms[sc3A.slot(l3A)].flags |= pfTapped
	if d := dynPayAsk(t, sc3A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// petals: the source itself yields two now, and the amount is still
	// indeterminate, so the gate opens with no other source on the board
	sc4 := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	l4 := sc4.perm(lotus, 0)
	sc4.s.perms[sc4.slot(l4)].setCtrN(sc4.s, int(kop.CtrPetal), 2)
	dynProbe(t, sc4)
	if !sc4.s.trigCostPotentialPays(sc4.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {6} beside a two-petal Lotus Blossom")
	}
	sc4A := newScenTableLib(t, tbl, isl, Config{NoElide: true})
	l4A := sc4A.perm(lotus, 0)
	sc4A.s.perms[sc4A.slot(l4A)].setCtrN(sc4A.s, int(kop.CtrPetal), 2)
	if d := dynPayAsk(t, sc4A, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("petals ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}
