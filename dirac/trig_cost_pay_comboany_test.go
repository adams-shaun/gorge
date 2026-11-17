package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// comboAnyPayTable builds a two-seat matchup holding Axebane Guardian, an
// unrestricted Combo Any source ("{T}: Add X mana in any combination of
// colors", the same census gap as Sage of the Maze), plus a synthetic 1/1
// whose attack trigger carries an optional own-cost ask: OpIfPaid {1} (C 17 =
// the PayMana kind with an empty skip nibble, as mana_rider_trigcost_test.go's
// rider), then draw a card.
func comboAnyPayTable(t *testing.T) (*kop.Table, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Axebane Guardian", Count: 4}, {Name: "Plains", Count: 8}}
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
		Name: "Test Combo Any Trigger Cost", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(0), C: 17},
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
			}}},
	})
	return tbl, id, trig
}

// comboAnyPayBoard is a scenario whose payer's only permanent besides the
// synthetic trigger is an untapped Combo Any source (tapped=false) or a
// tapped one (tapped=true); its pool is empty.
func comboAnyPayBoard(t *testing.T, tbl *kop.Table, source CardID, tapped bool) (*scen, Obj) {
	t.Helper()
	sc := newScenTableLib(t, tbl, source, Config{NoElide: true})
	o := sc.perm(source, 0)
	sl := sc.slot(o)
	if tapped {
		sc.s.perms[sl].flags |= pfTapped
	} else {
		sc.s.perms[sl].flags &^= pfTapped
	}
	return sc, o
}

// comboAnyPayAsk fires the synthetic attack trigger and walks to the next
// choose decision: the payer's trigger-cost pay ask.
func comboAnyPayAsk(t *testing.T, sc *scen, trig CardID) Decision {
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

// TestComboAnyTriggerCostPay: a trigger's own optional mana cost is offered on
// the payer's POTENTIAL mana (rules respay.go resPayNeedsMana over
// potential.go), and gorge's potential walk prices a Combo Any production
// ("{T}: Add X mana in any combination of colors", Axebane Guardian's Combo
// Any ability, lowered as a gfGapOther census gap, mana_comboany.go) unbounded
// in every colour: addPotentialMana's producedOpen. The kernel's offer gate
// (trigCostPotentialPays over anyManaSource and availPotentialFor) admitted a
// source only through canTapForManaPC, the each-colour gap, a storage battery,
// a rider or a restricted ability, so the Combo Any source was invisible and
// the ask was decline-only while gorge offered pay and decline. With the fix
// the gate takes caPotentialOpen first: the source is live, the elected pay's
// window walk (windowComboTap) funds the cost or the payer declines.
func TestComboAnyTriggerCostPay(t *testing.T) {
	tbl, id, trig := comboAnyPayTable(t)
	source := id["Axebane Guardian"]

	// live: the gate counts the source, and the ask offers pay beside decline
	sc, o := comboAnyPayBoard(t, tbl, source, false)
	sl := sc.slot(o)
	if !sc.k.gen.anyComboAny || !sc.k.isComboAny(sc.s.cardAt(sl)) {
		t.Fatal("precondition: Axebane Guardian is not the table's Combo Any gap")
	}
	for i := range sc.s.pl[0].pool {
		if sc.s.pl[0].pool[i] != 0 {
			t.Fatalf("precondition: pool colour %d is %d, want an empty pool", i, sc.s.pl[0].pool[i])
		}
	}
	sc.s.res.e.kind = seTrigger
	if !sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 1, 0, 0) {
		t.Fatal("trigCostPotentialPays refused the {1} beside the live Combo Any source")
	}

	// the ask, on a fresh state (the probe above pokes res): gorge poses the
	// trigger's own {1} beside the live Combo Any source
	scA, _ := comboAnyPayBoard(t, tbl, source, false)
	if d := comboAnyPayAsk(t, scA, trig); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live ask %d options %v, want decline then pay: gorge's potential counts the Combo Any source", d.Sub, d.Options[:d.N])
	}

	// tapped: gorge's walk skips the source, so decline only
	sc2, _ := comboAnyPayBoard(t, tbl, source, true)
	sc2.s.res.e.kind = seTrigger
	if sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 1, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {1} beside a tapped Combo Any source")
	}
	sc2A, _ := comboAnyPayBoard(t, tbl, source, true)
	if d := comboAnyPayAsk(t, sc2A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("tapped ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// absent: no source at all, decline only
	sc3 := newScenTableLib(t, tbl, source, Config{NoElide: true})
	sc3.s.res.e.kind = seTrigger
	if sc3.s.trigCostPotentialPays(sc3.k, 0, kop.PayMana, 0, 1, 0, 0) {
		t.Fatal("trigCostPotentialPays offered the {1} with no mana source")
	}
	sc3A := newScenTableLib(t, tbl, source, Config{NoElide: true})
	if d := comboAnyPayAsk(t, sc3A, trig); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("absent ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}
}
