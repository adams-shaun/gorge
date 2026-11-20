package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTurnUpStoreShortBatteryNoOps: the keyword turn-up's offer is gorge's
// potential read, which prices a storage battery's announced X unbounded even
// when it holds no counter (mana_storecensus.go storeCensusPays). Taking the
// action must re-prove the real payment first: activating a short battery
// makes less mana than the offer assumed, so the turn-up has to degrade to the
// no-op gorge's morphTurnUpPayableAs takes, not tap the battery and then ask
// the planner for a plan it cannot find. A battery holding one counter does pay
// Arno Dorian's Disguise {B}{R} beside a Swamp and must still turn up.
func TestTurnUpStoreShortBatteryNoOps(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Arno Dorian", Count: 1},
		{Name: "Mercadian Bazaar", Count: 1},
		{Name: "Swamp", Count: 20},
	}
	tbl, ids, _, err := cardsettest.Synth(t, "tail-26").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range ids {
		id[tbl.Cards[c].Name] = c
	}
	for _, name := range []string{"Arno Dorian", "Mercadian Bazaar", "Swamp"} {
		if id[name] == NoCard {
			t.Fatalf("the probe card %s is missing from the tail-26 table", name)
		}
	}
	for _, tc := range []struct {
		name  string
		ctrs  int
		turns bool
	}{
		{"a battery holding no counter no-ops", 0, false},
		{"a battery holding one counter pays", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
			bazaar := sc.perm(id["Mercadian Bazaar"], 0)
			bazaarSl := sc.slot(bazaar)
			sc.s.perms[bazaarSl].flags &^= pfTapped // Mercadian Bazaar enters tapped
			if tc.ctrs > 0 {
				sc.s.perms[bazaarSl].addCtrN(sc.s, int(kop.CtrStorage), tc.ctrs)
			}
			sc.perm(id["Swamp"], 0)
			arno := sc.obj(id["Arno Dorian"], 0)
			arnoSl := sc.s.enterFaceDown(sc.k, arno, 0, kop.MorphDisguise)

			// Preconditions: the board is the one the store arm prices, and
			// neither the Swamp alone nor the planner pays the Disguise cost.
			if got := sc.s.perms[bazaarSl].ctrN(sc.s, int(kop.CtrStorage)); got != tc.ctrs {
				t.Fatalf("precondition: the battery holds %d counters, want %d", got, tc.ctrs)
			}
			if !sc.s.faceDown(sc.k, arnoSl) {
				t.Fatalf("precondition: Arno Dorian is not face down")
			}
			cost, _, _, ok := sc.s.turnUpCost(sc.k, arnoSl, 0)
			if !ok {
				t.Fatalf("precondition: the Disguise turn-up has no cost")
			}
			c0 := cost.AtX(0)
			if sc.s.sourcesPay(sc.k, 0, &c0) {
				t.Fatalf("precondition: the untapped sources alone pay %+v", c0)
			}
			if !sc.s.storeCensusPays(sc.k, 0, &c0) {
				t.Fatalf("precondition: the battery's potential read does not price %+v", c0)
			}
			if n := sc.turnUpOffers(0); n != 1 {
				t.Fatalf("the potential turn-up is not offered: %d options", n)
			}

			sc.s.turnFaceUp(sc.k, arno, 0)

			if got := sc.s.faceDown(sc.k, arnoSl); got == tc.turns {
				t.Fatalf("face down after the turn-up = %v, want %v", got, !tc.turns)
			}
			if got := sc.s.perms[bazaarSl].flags&pfTapped != 0; got == !tc.turns {
				t.Fatalf("battery tapped = %v, want %v", got, tc.turns)
			}
			if got := sc.s.perms[bazaarSl].ctrN(sc.s, int(kop.CtrStorage)); got != 0 {
				t.Fatalf("the battery left %d storage counters, want 0", got)
			}
		})
	}
}
