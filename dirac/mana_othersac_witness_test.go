package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOtherSacWitnessIsPerPayment: the planned other-sacrifice witness log
// (planSacLog) bounds one plan's steps (planSacSteps), not a game's. Six
// payments in a row on one live State, each an Insidious Fungus activation
// paid by one planned Ashnod's Altar step, never stall the kernel, and the
// log holds only the last payment's pairing.
func TestOtherSacWitnessIsPerPayment(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Ashnod's Altar", Count: 1}, {Name: "Insidious Fungus", Count: 1},
		{Name: "Syr Konrad, the Grim", Count: 1}, {Name: "Plains", Count: 20},
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, GorgeAutopaySurface: true})
	altar := sc.perm(id["Ashnod's Altar"], 0)
	ab := -1
	for i, a := range tbl.Cards[id["Insidious Fungus"]].Abilities {
		if a.Kind == kop.AbActivated && a.Cost.Flags&kop.CostSacSelf != 0 && a.Cost.Mana.MV() == 2 {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("precondition: Insidious Fungus has no {2}, Sacrifice it activation")
	}
	s := sc.s
	if s.planSacLogOf() == nil {
		t.Fatal("precondition: the table has no planned other-sacrifice witness log")
	}
	const payments = planSacSteps + 2
	for i := 0; i < payments; i++ {
		fungus := sc.perm(id["Insidious Fungus"], 0)
		konrad := sc.perm(id["Syr Konrad, the Grim"], 0)
		if !sc.activateAb(fungus, uint8(ab)) {
			t.Fatalf("payment %d: the Fungus's activation is not offered beside the altar", i)
		}
		sc.settle(t)
		if s.g.stalled != 0 {
			t.Fatalf("payment %d: the kernel stalled (%d): the witness log outlived its payment", i, s.g.stalled)
		}
		if z := s.objs[konrad].zone; z != kop.ZoneGraveyard {
			t.Fatalf("payment %d: Syr Konrad is in zone %d, want the graveyard (the planned step's victim)", i, z)
		}
		if z := s.objs[altar].zone; z != kop.ZoneBattlefield {
			t.Fatalf("payment %d: the altar is in zone %d, want the battlefield", i, z)
		}
		if srcs, vics := s.PlanSacWitness(); len(srcs) > 1 || len(vics) > 1 {
			t.Fatalf("payment %d: the witness holds %d pairings, want at most the last payment's one", i, len(srcs))
		}
	}
}
