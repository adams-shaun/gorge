package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTurnUpStoreShortBatteryNoOps: the keyword turn-up's offer is gorge's
// potential play, which announces a storage battery's X against the counters
// it holds (mana_storecensus.go storeCensusPays): a battery holding none funds
// nothing, its plan is "insufficient", and the turn-up is not offered (tail-67
// pair 44, Fountain of Cho with no counter beside Basilica Stalker's {4}{B}).
// Taking the action anyway must still degrade to the no-op gorge's
// morphTurnUpPayableAs takes, not tap the battery and then ask the planner for
// a plan it cannot find. A battery holding one counter does pay a Morph
// {1}{B} beside a Plains and must still turn up. The battery is
// Bottomless Vault from the storagex pool and the morph creature is built
// here, so the test reads no raw table.
func TestTurnUpStoreShortBatteryNoOps(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bottomless Vault", Count: 40}}
	tbl, ids, _, err := cardsettest.Set(t, "storagex").Matchup(deck, []cardset.Entry{{Name: "Plains", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	var vault CardID = NoCard
	for _, c := range ids {
		if tbl.Cards[c].Name == "Bottomless Vault" {
			vault = c
		}
	}
	if vault == NoCard {
		t.Fatalf("Bottomless Vault is missing from the storagex table")
	}
	var plains CardID = NoCard
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	if plains == NoCard {
		t.Fatalf("the storagex table has no Plains")
	}
	morph := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Morph Probe",
		Types: kop.TCreature,
		Power: 2, Toughness: 2,
		MorphFamily: kop.MorphMorph,
		MorphCost:   kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.B: 1}},
	})
	for _, tc := range []struct {
		name  string
		ctrs  int
		turns bool
	}{
		{"a battery holding no counter is not offered and no-ops", 0, false},
		{"a battery holding one counter pays", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
			vaultPerm := sc.perm(vault, 0)
			vaultPermSl := sc.slot(vaultPerm)
			sc.s.perms[vaultPermSl].flags &^= pfTapped // Bottomless Vault enters tapped
			if tc.ctrs > 0 {
				sc.s.perms[vaultPermSl].addCtrN(sc.s, int(kop.CtrStorage), tc.ctrs)
			}
			sc.perm(plains, 0)
			probe := sc.obj(morph, 0)
			probeSl := sc.s.enterFaceDown(sc.k, probe, 0, kop.MorphMorph)

			// Preconditions: the board is the one the store arm prices, and
			// neither the Plains alone nor the planner pays the Morph cost.
			if got := sc.s.perms[vaultPermSl].ctrN(sc.s, int(kop.CtrStorage)); got != tc.ctrs {
				t.Fatalf("precondition: the battery holds %d counters, want %d", got, tc.ctrs)
			}
			if !sc.s.faceDown(sc.k, probeSl) {
				t.Fatalf("precondition: Morph Probe is not face down")
			}
			cost, _, _, ok := sc.s.turnUpCost(sc.k, probeSl, 0)
			if !ok {
				t.Fatalf("precondition: the Morph turn-up has no cost")
			}
			c0 := cost.AtX(0)
			if sc.s.sourcesPay(sc.k, 0, &c0) {
				t.Fatalf("precondition: the untapped sources alone pay %+v", c0)
			}
			if got := sc.s.storeCensusPays(sc.k, 0, &c0); got != tc.turns {
				t.Fatalf("the battery's potential read prices %+v = %v, want %v", c0, got, tc.turns)
			}
			want := 0
			if tc.turns {
				want = 1
			}
			if n := sc.turnUpOffers(0); n != want {
				t.Fatalf("the potential turn-up offers = %d options, want %d", n, want)
			}

			sc.s.turnFaceUp(sc.k, probe, 0)

			if got := sc.s.faceDown(sc.k, probeSl); got == tc.turns {
				t.Fatalf("face down after the turn-up = %v, want %v", got, !tc.turns)
			}
			if got := sc.s.perms[vaultPermSl].flags&pfTapped != 0; got == !tc.turns {
				t.Fatalf("battery tapped = %v, want %v", got, tc.turns)
			}
			if got := sc.s.perms[vaultPermSl].ctrN(sc.s, int(kop.CtrStorage)); got != 0 {
				t.Fatalf("the battery left %d storage counters, want 0", got)
			}
		})
	}
}
