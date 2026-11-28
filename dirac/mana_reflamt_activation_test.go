package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflAmtBesideItsGrantOffersTheActivationItsColourPays: Incubation Druid
// with three +1/+1 counters also holds Rishkar's granted "{T}: Add {G}". The
// granted view stood for the source in the potential bound, so the Druid's
// reflected colours were dropped and Falkenrath Exterminator's {2}{R}, paid by
// the Plains and the Druid's {R} (a tapped Mountain "could produce" it), was
// not offered. Each mana ability counts on its own (CR 605.1a). With no land
// that could make {R} the activation stays unoffered.
func TestReflAmtBesideItsGrantOffersTheActivationItsColourPays(t *testing.T) {
	for _, tc := range []struct {
		tapped string
		want   bool
	}{{"Mountain", true}, {"Forest", false}} {
		deck := []cardset.Entry{
			{Name: "Falkenrath Exterminator", Count: 1}, {Name: "Incubation Druid", Count: 1}, {Name: "Rishkar, Peema Renegade", Count: 1},
			{Name: "Plains", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Forest", Count: 1},
		}
		tbl, da, _, err := edhSet(t).Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		cfg := LockstepConfig(SurfaceAutopay)
		cfg.NoElide = true
		sc := newScenTableLib(t, tbl, id["Forest"], cfg)
		falk := sc.perm(id["Falkenrath Exterminator"], 0)
		land := sc.perm(id[tc.tapped], 0)
		sc.s.perms[sc.slot(land)].flags |= pfTapped
		sc.perm(id["Plains"], 0)
		druid := sc.perm(id["Incubation Druid"], 0)
		sc.s.perms[sc.slot(druid)].addCtrN(sc.s, int(kop.CtrP1P1), 3)
		sc.sickPerm(id["Rishkar, Peema Renegade"], 0)
		if !sc.k.isReflAmt(id["Incubation Druid"]) || !sc.s.grantGapLive(sc.k, 0) || sc.s.grantIdx(sc.k, sc.slot(druid)) < 0 {
			t.Fatal("precondition: the Druid is not a reflected Amount$ source holding a live granted mana ability")
		}
		if got := sc.activateOffered(falk, 0); got != tc.want {
			t.Errorf("tapped %s: Falkenrath Exterminator's activation offered = %v, want %v", tc.tapped, got, tc.want)
		}
	}
}
