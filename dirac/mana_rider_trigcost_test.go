package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// woodlotTrigTable builds the manarider pool table plus a synthetic 1/1 whose
// attack trigger carries an optional own-cost ask: OpIfPaid {1}{B} (64 = one
// {B} pip, three bits per colour, C 17 = the PayMana kind), then draw a card.
func woodlotTrigTable(t *testing.T) (*kop.Table, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "manarider")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	trig := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Rider Trigger Cost", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(64), C: 17},
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
			}}},
	})
	return tbl, id, trig
}

// woodlotBoard puts an untapped, two-counter Peat Bog on p0's battlefield.
func woodlotBoard(t *testing.T) (*scen, map[string]CardID, CardID, Obj) {
	t.Helper()
	tbl, id, trig := woodlotTrigTable(t)
	sc := newScenTableLib(t, tbl, id["Peat Bog"], Config{NoElide: true})
	o := sc.perm(id["Peat Bog"], 0)
	sl := sc.slot(o)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok || a.ManaRider != kop.ManaRiderDeplete {
		t.Fatal("precondition: Peat Bog is not the depletion-rider shape")
	}
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
	if !sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 2) {
		t.Fatal("precondition: could not put the two depletion counters on")
	}
	sc.s.perms[sl].flags &^= pfTapped // it entered tapped; it untaps on its controller's turn
	return sc, id, trig, o
}

// TestManaWindowCounterRemovalSource: a trigger's own optional mana cost is
// offered on the payer's POTENTIAL mana (rules respay.go resPayNeedsMana over
// potential.go), and gorge's potential walk counts a bounded rider mana
// source (Hickory Woodlot's "{T}, Remove a depletion counter: Add {G}{G}",
// lowered as a Deplete rider, mana_rider.go). The kernel's offer gate
// (trigCostPotentialPays over anyManaSource and availPotentialFor) used to
// require the census tap flag, whose exactly-{T} test a Tap+SubCounter cost
// fails, so the source was invisible and the ask was decline-only while
// gorge offered pay and decline. With the fix the gate counts the rider and
// the elected pay's window walk (windowTapNoOwed) takes it the way the
// manual window does: the counter comes off, the land taps, its mana floats,
// and the trigger body runs.
func TestManaWindowCounterRemovalSource(t *testing.T) {
	sc, _, trig, o := woodlotBoard(t)
	sl := sc.slot(o)

	// the potential walk counts the rider's {B}{B} where canTapForManaPC
	// skipped the source
	other := payDesc{class: pdOther}
	var pot avail
	sc.s.availPotentialFor(sc.k, 0, &pot, &other)
	if pot.total != 2 {
		t.Fatalf("potential units %d, want the rider's two {B}", pot.total)
	}
	seen := uint8(0)
	for i, n := range pot.cnt {
		if n > 0 {
			seen |= pot.mask[i]
		}
	}
	if seen != 1<<kop.B {
		t.Fatalf("potential colours %d, want {B} only", seen)
	}
	if !sc.s.anyManaSource(sc.k, 0, &other) {
		t.Fatal("anyManaSource skipped the rider source")
	}

	// the ask: gorge offers the trigger's own {1}{B} on the rider's potential
	tr := sc.perm(trig, 0)
	sc.s.fireObj(sc.k, kop.TrigAttacks, sc.slot(tr), tr, 0, 0)
	var d Decision
	for i := 0; i < 12; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if d.Kind != DkChoose || d.Sub != ChGenPay {
		t.Fatalf("decision kind %d sub %d, want the pay ask", d.Kind, d.Sub)
	}
	if int(d.N) != 2 {
		t.Fatalf("options %v (%d), want decline then pay: gorge's potential counts the rider source", d.Options[:d.N], d.N)
	}
	if d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("options %v, want decline then pay", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1]) // elect pay: the window walk funds it through the rider
	a, _ := sc.s.riderAbilityOf(sc.k, sl)
	if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 1 {
		t.Fatalf("depletion counters %d after the pay, want 1", n)
	}
	if !sc.tapped(o) {
		t.Fatal("Peat Bog untapped: the window walk did not activate the rider")
	}
	if got := sc.s.pl[0].pool[kop.B]; got != 0 {
		t.Fatalf("{B} in pool %d, want 0 ({B}{B} floated, {1}{B} spent both)", got)
	}
	if sc.s.pl[0].handN != 1 {
		t.Fatalf("hand %d, want 1: the paid cost ran the trigger body", sc.s.pl[0].handN)
	}
	if !sc.onBF(o) {
		t.Fatal("Peat Bog left the battlefield after one activation")
	}
}
