package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An Aura parked for its order ask does not move with the pass's other casualty,
// which no competing source covers: Misery's Shadow and Stone of Erech reach only
// their opponent's creatures, so the table makes the bear an Aura as well (CR 616.1).
func TestParkedAuraBesideOtherCasualty(t *testing.T) {
	deck := []cardset.Entry{{Name: "Misery's Shadow", Count: 1}, {Name: "Stone of Erech", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Walking Corpse", Count: 1}, {Name: "Forest", Count: 1}}
	tbl, da, _, err := cardsettest.Set(t, "exileanddoublers").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	cp.Cards[id["Grizzly Bears"]].Types |= kop.TAura
	sc := newScenTableLib(t, &cp, id["Forest"], Config{NoElide: true})
	s, k := sc.s, sc.k
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc.perm(id["Misery's Shadow"], 1)
	sc.perm(id["Stone of Erech"], 1)
	victim := sc.perm(id["Walking Corpse"], 1)
	s.perms[sc.slot(victim)].damage = 9
	aura := sc.perm(id["Grizzly Bears"], 0)
	s.perms[sc.slot(aura)].attached = sc.slot(victim)
	if !s.sba(k) || s.ctl != ctlExileOrder {
		t.Fatalf("no order ask: ctl %d, stalled %v", s.ctl, s.Stalled())
	}
	if s.objs[victim].zone != kop.ZoneGraveyard {
		t.Fatalf("the other casualty is in zone %d at the ask, want the graveyard", s.objs[victim].zone)
	}
	if s.objs[aura].zone != kop.ZoneBattlefield {
		t.Fatalf("the parked Aura is in zone %d at the ask, want the battlefield", s.objs[aura].zone)
	}
}
