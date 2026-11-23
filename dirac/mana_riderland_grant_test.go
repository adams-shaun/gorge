package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// riderLandGrantScen builds p0's board over the MBC pool: Peat Bog (two depletion
// counters, untapped) wearing the opponent's Abundant Growth, a Kavu Predator
// (the creature the kicker sacrifices), the named lands, and a Primal Growth
// in hand.
func riderLandGrantScen(t *testing.T, cfg Config, lands ...string) (*scen, Obj) {
	t.Helper()
	deck0 := []cardset.Entry{{Name: "Primal Growth", Count: 1}, {Name: "Kavu Predator", Count: 1}, {Name: "Peat Bog", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}}
	deck1 := []cardset.Entry{{Name: "Abundant Growth", Count: 2}, {Name: "Peat Bog", Count: 1}, {Name: "Forest", Count: 8}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck0, deck1)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Forest"], cfg)
	bog := sc.perm(id["Peat Bog"], 0)
	sl := sc.slot(bog)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Peat Bog is not the depletion-rider shape")
	}
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
	if !sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 2) {
		t.Fatal("precondition: could not put the two depletion counters on")
	}
	sc.s.perms[sl].flags &^= pfTapped
	aura := sc.perm(id["Abundant Growth"], 1) // the opponent's Aura on p0's land
	sc.s.attachTo(sc.k, sc.slot(aura), sl)
	sc.s.refreshTy(sc.k)
	for _, n := range append([]string{"Kavu Predator"}, lands...) {
		sc.perm(id[n], 0)
	}
	return sc, sc.hand(id["Primal Growth"], 0)
}

// kickedOffered reports whether the kicked cast (mode 1) of o is on offer.
func kickedOffered(s *State, o Obj) bool {
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode()&3 == 1 {
			return true
		}
	}
	return false
}

// TestRiderLandGrantIsOneTap: Peat Bog's own {T}, remove a counter: {B}{B} and
// Abundant Growth's granted {T}: one mana of any colour share the one tap
// (CR 605.3b), so Peat Bog and a Plains make {B}{B}{W} or {G}{W}, never the {G}
// plus a third mana that Primal Growth's kicked {2}{G} needs; a Forest beside
// them pays it (Peat Bog's {B}{B}, the Plains and the Forest's {G}).
func TestRiderLandGrantIsOneTap(t *testing.T) {
	for _, sf := range []Surface{SurfaceAutopay, SurfaceFull} {
		cfg := lockCfg(sf)
		sc, pg := riderLandGrantScen(t, cfg, "Plains", "Forest")
		if !kickedOffered(sc.s, pg) {
			t.Errorf("surface %d: precondition: Primal Growth kicked is not offered with a Forest beside Peat Bog and a Plains", sf)
		}
		sc, pg = riderLandGrantScen(t, cfg, "Plains")
		if kickedOffered(sc.s, pg) {
			t.Errorf("surface %d: Primal Growth kicked offered from Peat Bog (+Abundant Growth) and a Plains: {B}{B}{W} or {G}{W}, never {2}{G}", sf)
		}
	}
}

func lockCfg(sf Surface) Config {
	c := LockstepConfig(sf)
	c.NoElide = true
	return c
}
