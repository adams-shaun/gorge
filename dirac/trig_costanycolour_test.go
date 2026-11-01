package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTriggerCostPotentialAnyColour: gorge offers a trigger's own optional
// mana cost on the payer's POTENTIAL mana, and its potential census folds a
// "Produced$ Any" source to unbounded in every colour even when its amount
// is a known one (potential.go:324/353). A Cultivator's Caravan (any-colour
// source) therefore makes an unaffordable {1}{R}{G} trigger cost look
// payable: gorge offers pay + decline, the elected pay taps the sources,
// the charge fails and the flow declines (CR 118.3). The kernel emulates
// the over-offer (execPay's trigCostPotentialPays), the manual-window
// fallback (payReplanned), and the window answer for a plain any-colour
// source (windowCol: W, as the oracle's payWindowAnswer reads no Owed).
func TestTriggerCostPotentialAnyColour(t *testing.T) {
	// case is one board: p0's permanents and the expected pay ask.
	cases := []struct {
		name string
		bf   []string
		n    int // expected option count at ChGenPay
		pool [kop.NumManaKinds]uint8
	}{
		// the tapped Caravan no longer counts, so {1}{R}{G} is out of reach and
		// the fund loop stops before the Swamp (rules resPayFund)
		{"caravan-and-swamp", []string{"Cultivator's Caravan", "Swamp"}, 2,
			[kop.NumManaKinds]uint8{kop.W: 1}},
		{"caravan-alone", []string{"Cultivator's Caravan"}, 2,
			[kop.NumManaKinds]uint8{kop.W: 1}},
		{"swamp-and-plains", []string{"Swamp", "Plains"}, 1, [kop.NumManaKinds]uint8{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl, id, trig := vehTrigTable(t)
			sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
			for _, name := range tc.bf {
				sc.perm(id[name], 0)
			}
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
			if int(d.N) != tc.n {
				t.Fatalf("options %v (%d), want %d: gorge's potential over-offer", d.Options[:d.N], d.N, tc.n)
			}
			if tc.n == 1 {
				return // decline only: no pay route to elect
			}
			if d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
				t.Fatalf("options %v, want decline then pay", d.Options[:d.N])
			}
			sc.s.Apply(d.Options[1]) // elect pay: gorge taps, fails the charge, declines
			if got := sc.s.pl[0].pool; got != tc.pool {
				t.Fatalf("pool %v, want %v (the failed charge leaves its floated mana)", got, tc.pool)
			}
			if sc.s.pl[0].handN != 0 {
				t.Fatalf("hand %d, want 0: the failed pay skips the trigger body", sc.s.pl[0].handN)
			}
		})
	}
}

// vehTrigTable builds the czaltdest pool table plus a synthetic 1/1 whose
// attack trigger is Balthier and Fran's cost shape: OpIfPaid {1}{R}{G}
// (4608 = one R pip and one G pip packed three bits per colour, C 17 = the
// PayMana kind with the one-instruction skip when unpaid), then draw a card.
func vehTrigTable(t *testing.T) (*kop.Table, map[string]CardID, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "czaltdest")
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
		Name: "Test Vehicle Trigger Cost", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpIfPaid, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(4608), C: 17},
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
			}}},
	})
	return tbl, id, trig
}
