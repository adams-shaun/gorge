package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// hostSacScen: Hill Giant equipped with Deconstruction Hammer (its granted
// "{3}, {T}, Sacrifice this Equipment: destroy target artifact or
// enchantment") and a second Hammer to aim at, three mana in the pool.
func hostSacScen(t *testing.T) (sc *scen, giant, hammer, other Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Deconstruction Hammer", Count: 8}, {Name: "Hill Giant", Count: 10}, {Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11}}
	set := cardsettest.Set(t, "sachost")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	giant = sc.perm(id["Hill Giant"], 0)
	hammer = sc.perm(id["Deconstruction Hammer"], 0)
	other = sc.perm(id["Deconstruction Hammer"], 1)
	sc.s.perms[sc.slot(hammer)].attached = sc.slot(giant)
	sc.s.perms[sc.slot(giant)].flags &^= pfSick
	return sc, giant, hammer, other
}

// TestGrantedSacHostCost: a granted ability whose cost sacrifices the
// granting Equipment is offered to the Equipment's controller, and paying it
// taps the creature, spends the mana and sends the Equipment to the graveyard
// (CR 701.21a). Without the cost part the card is not lowered, so the pool
// has no Hammer at all.
func TestGrantedSacHostCost(t *testing.T) {
	sc, giant, hammer, _ := hostSacScen(t)
	s := sc.s
	s.pl[0].pool[kop.W] = 3
	d := s.Step()
	a, ok := grantedAct(&d, giant)
	if !ok {
		t.Fatalf("the granted ability is not offered with {3} in the pool: %v", d.Options[:d.N])
	}
	if a.Target() != TCode(uint8(hammer)) {
		t.Fatalf("granted ability host %d, want the Equipment %d", a.Target(), hammer)
	}
	s.Apply(a)
	for i := 0; i < 8 && s.objs[hammer].zone == kop.ZoneBattlefield; i++ {
		d = s.Step()
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0])
	}
	if z := s.objs[hammer].zone; z != kop.ZoneGraveyard {
		t.Fatalf("Deconstruction Hammer is in zone %d after its granted cost, want the graveyard", z)
	}
	if s.perms[sc.slot(giant)].flags&pfTapped == 0 {
		t.Fatal("the equipped creature is untapped after paying {T}")
	}
}

// TestGrantedSacHostNotOffered: the host must be the payer's own permanent
// (CR 701.21a): with the Equipment under the opponent's control the equipped
// creature's controller cannot pay the sacrifice.
func TestGrantedSacHostNotOffered(t *testing.T) {
	sc, giant, hammer, _ := hostSacScen(t)
	s := sc.s
	s.pl[0].pool[kop.W] = 3
	s.perms[sc.slot(hammer)].ctrl = 1
	d := s.Step()
	if a, ok := grantedAct(&d, giant); ok {
		t.Fatalf("granted ability %d offered with the Equipment under the opponent's control", a.Mode())
	}
}
