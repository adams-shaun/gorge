package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// hostExileScen is hostSacScen with Deconstruction Hammer's granted cost
// turned into Exile<1/OriginalHost> (kop.CostHostExile), the shape The
// Dominion Bracelet's granted ability lowers to (no corpus card with that cost
// lowers whole: its effect, ControlPlayer, is refused by design -- see
// TestControlPlayerRefusedByDesign).
func hostExileScen(t *testing.T) (sc *scen, giant, hammer Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Deconstruction Hammer", Count: 8}, {Name: "Hill Giant", Count: 10}, {Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11}}
	set := cardsettest.Set(t, "sachost")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			if c := &tbl.Cards[i].Abilities[j].Cost; c.Host == kop.CostHostSac {
				c.Host = kop.CostHostExile
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("no granted host-sacrifice cost to turn into a host exile")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	giant = sc.perm(id["Hill Giant"], 0)
	hammer = sc.perm(id["Deconstruction Hammer"], 0)
	sc.s.perms[sc.slot(hammer)].attached = sc.slot(giant)
	sc.s.perms[sc.slot(giant)].flags &^= pfSick
	return sc, giant, hammer
}

// TestGrantedExileHostCost: a granted ability whose cost exiles the granting
// Equipment moves it to exile, not the graveyard (CR 118.3, 602.2b): it is
// not sacrificed.
func TestGrantedExileHostCost(t *testing.T) {
	sc, giant, hammer := hostExileScen(t)
	s := sc.s
	s.pl[0].pool[kop.W] = 3
	d := s.Step()
	a, ok := grantedAct(&d, giant)
	if !ok {
		t.Fatalf("the granted ability is not offered with {3} in the pool: %v", d.Options[:d.N])
	}
	s.Apply(a)
	for i := 0; i < 8 && s.objs[hammer].zone == kop.ZoneBattlefield; i++ {
		d = s.Step()
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0])
	}
	if z := s.objs[hammer].zone; z != kop.ZoneExile {
		t.Fatalf("Deconstruction Hammer is in zone %d after its granted exile cost, want exile (%d)", z, kop.ZoneExile)
	}
	if s.perms[sc.slot(giant)].flags&pfTapped == 0 {
		t.Fatal("the equipped creature is untapped after paying {T}")
	}
}

// TestGrantedExileHostNotOffered: the host must be the payer's own permanent:
// with the Equipment under the opponent's control the ability is not offered.
func TestGrantedExileHostNotOffered(t *testing.T) {
	sc, giant, hammer := hostExileScen(t)
	s := sc.s
	s.pl[0].pool[kop.W] = 3
	s.perms[sc.slot(hammer)].ctrl = 1
	d := s.Step()
	if a, ok := grantedAct(&d, giant); ok {
		t.Fatalf("granted ability %d offered with the Equipment under the opponent's control", a.Mode())
	}
}
