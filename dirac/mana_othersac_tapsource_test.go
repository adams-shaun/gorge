package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// otherSacTapSourceScen is seat 0's main phase with an Ashnod's Altar and a
// Formidable Speaker ({1},{T}: untap another target permanent) on its side,
// plus the named extra permanents; it returns the scenario, the Speaker and
// the index of its {T} activation.
func otherSacTapSourceScen(t *testing.T, extra ...string) (*scen, Obj, uint8) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Ashnod's Altar", Count: 1}, {Name: "Formidable Speaker", Count: 1},
		{Name: "Carrion Feeder", Count: 1}, {Name: "Plains", Count: 20},
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
	sc.perm(id["Ashnod's Altar"], 0)
	speaker := sc.perm(id["Formidable Speaker"], 0)
	for _, n := range extra {
		sc.perm(id[n], 0)
	}
	ab := -1
	for i, a := range tbl.Cards[id["Formidable Speaker"]].Abilities {
		if a.Kind == kop.AbActivated && a.Cost.Flags&kop.CostTap != 0 && a.Cost.Mana.MV() == 1 {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("precondition: Formidable Speaker has no {1},{T} activation")
	}
	return sc, speaker, uint8(ab)
}

// TestOtherSacPlanActivationKeepsTheTappedSource: gorge's activation planner
// reserves a {T}/{Q} cost's source beside the kept objects before it assigns
// plan victims (planComposedCostFor's tapped, pay.AssignPlanVictims), so the
// Speaker, which taps to activate, is no Ashnod's Altar victim. With the
// Speaker the altar's only fodder there is no plan for its {1}, and the
// activation is not offered; a Carrion Feeder beside it is the victim.
func TestOtherSacPlanActivationKeepsTheTappedSource(t *testing.T) {
	t.Run("the only fodder is the tapping source", func(t *testing.T) {
		sc, speaker, ab := otherSacTapSourceScen(t)
		if sc.activateOffered(speaker, ab) {
			t.Fatal("the activation is offered: its altar unit sacrifices the Speaker its own {T} taps")
		}
	})
	t.Run("another creature is the victim", func(t *testing.T) {
		sc, speaker, ab := otherSacTapSourceScen(t, "Carrion Feeder")
		if !sc.activateOffered(speaker, ab) {
			t.Fatal("the activation is withheld: the Carrion Feeder is the altar's victim")
		}
	})
}
