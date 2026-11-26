package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// triggerXColourTable is a synthetic attack trigger whose own cost is PayManaX
// with XColor$ red (kop.PayXColorB(kop.R)), the Leyline Tyrant shape: "you
// may pay any amount of {R}".
func triggerXColourTable(t *testing.T) (*kop.Table, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Forest", Count: 20}}
	tbl, ids, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range ids {
		id[tbl.Cards[c].Name] = c
	}
	if id["Mountain"] == NoCard || id["Forest"] == NoCard {
		t.Fatal("the probe lands are missing from the table")
	}
	trig := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Colour X Trigger", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(trigXCap),
					B: kop.Const(int(kop.PayXColorB(kop.R))), C: kop.PayManaX | 1<<4},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)},
			}}},
	})
	return tbl, id, trig
}

// triggerXColourAsk fires the synthetic attack trigger and steps to the node's
// first choose decision (the X announcement).
func triggerXColourAsk(t *testing.T, sc *scen, trig CardID) Decision {
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

// TestTriggerXColourBound: a trigger's own coloured X is bounded by the
// colour's supply, not the board's total mana. Two Mountains and two Forests
// offer X = 0..2; a generic X would offer 0..4.
func TestTriggerXColourBound(t *testing.T) {
	tbl, id, trig := triggerXColourTable(t)
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	d := triggerXColourAsk(t, sc, trig)
	if d.Sub != ChTrigX {
		t.Fatalf("first ask sub %d, want the X announcement", d.Sub)
	}
	if got := d.Options[d.N-1].Mode(); got != 2 {
		t.Fatalf("largest X offered %d, want 2 (two red sources; four mana on the board)", got)
	}
}

// TestTriggerXColourPayChargesPips: the elected coloured X is paid with that
// colour's pips, not generic mana. With two red and two green sources, X = 2
// taps only the Mountains; a generic cost would have tapped either pair.
func TestTriggerXColourPayChargesPips(t *testing.T) {
	tbl, id, trig := triggerXColourTable(t)
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	m1 := sc.perm(id["Mountain"], 0)
	m2 := sc.perm(id["Mountain"], 0)
	f1 := sc.perm(id["Forest"], 0)
	f2 := sc.perm(id["Forest"], 0)
	d := triggerXColourAsk(t, sc, trig)
	if d.Sub != ChTrigX || d.N < 3 {
		t.Fatalf("X ask sub %d with %d options, want 0..2", d.Sub, d.N)
	}
	sc.s.Apply(d.Options[2]) // X = 2
	// the pay / don't-pay election follows the announcement
	var pay Decision
	for i := 0; i < 12; i++ {
		pay = sc.s.Step()
		if pay.Kind == DkChoose {
			break
		}
		if pay.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if pay.Kind != DkChoose || pay.Sub != ChGenPay {
		t.Fatalf("decision kind %d sub %d, want the pay ask", pay.Kind, pay.Sub)
	}
	// the pay option is the last one (decline first)
	sc.s.Apply(pay.Options[pay.N-1])
	for i := 0; i < 20; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
	}
	tapped := func(o Obj) bool { return sc.s.perms[sc.slot(o)].flags&pfTapped != 0 }
	if !tapped(m1) || !tapped(m2) {
		t.Errorf("Mountains tapped = %v/%v, want both (the red pips)", tapped(m1), tapped(m2))
	}
	if tapped(f1) || tapped(f2) {
		t.Errorf("Forests tapped = %v/%v, want neither (a coloured X spends no green)", tapped(f1), tapped(f2))
	}
}
