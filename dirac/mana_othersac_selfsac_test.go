package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOtherSacPlanActivationPaysWithAnotherVictim: an activation whose cost
// sacrifices its own source (Insidious Fungus: {2}, Sacrifice it) and is paid
// by the planned Ashnod's Altar step keeps the source out of the altar's
// victims (gorge planCostParts' selfKept, pay.AssignPlanVictims; CR 601.2h,
// 602.2b): the Fungus is the lower-ranked creature, so an unreserved ranking
// would name it, but the altar sacrifices Syr Konrad and the Fungus is paid
// as the ability's own cost. The test executes the activation, not only its
// offer.
func TestOtherSacPlanActivationPaysWithAnotherVictim(t *testing.T) {
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
	fungus := sc.perm(id["Insidious Fungus"], 0)
	konrad := sc.perm(id["Syr Konrad, the Grim"], 0)
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
	if !sc.planSacVictimLessObj(fungus, konrad) {
		t.Fatal("precondition: the Fungus does not rank before Syr Konrad as a victim")
	}
	if !sc.activateAb(fungus, uint8(ab)) {
		t.Fatal("precondition: the Fungus's activation is not offered beside the altar")
	}
	sc.settle(t)
	if z := s.objs[konrad].zone; z != kop.ZoneGraveyard {
		t.Fatalf("Syr Konrad is in zone %d, want the graveyard: the altar's victim is another creature", z)
	}
	if z := s.objs[fungus].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the Fungus is in zone %d, want the graveyard (its own cost)", z)
	}
	if z := s.objs[altar].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the altar is in zone %d, want the battlefield", z)
	}
}

// planSacVictimLessObj is planSacVictimLess over two battlefield objects.
func (sc *scen) planSacVictimLessObj(x, y Obj) bool {
	return sc.s.planSacVictimLess(sc.k, sc.slot(x), sc.slot(y))
}
