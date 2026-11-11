package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Hand of Emrakul lockstep: four Eldrazi Spawn sacrificed rather than
// pay {9} (a two-subtype sacrifice filter, gorge AltCostIndex 1), then its
// Annihilator 1 attack trigger (CR 702.86a: the defending player sacrifices
// a permanent of their choice). The expected answer at every decision is
// gorge's.
var khandofemrakulDecks = [2][]cardset.Entry{
	{
		{Name: "Hand of Emrakul", Count: 8}, {Name: "Awakening Zone", Count: 6}, {Name: "Nest Invader", Count: 8},
		{Name: "Eldrazi Skyspawner", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Forest", Count: 20},
	},
}

func TestHandOfEmrakulLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "handofemrakul", khandofemrakulDecks, 600, "KHANDOFEMRAKUL_SEEDS")
}

// TestHandOfEmrakulLockstepMutation: with the alternative cost dropped some
// seed diverges (the pool plays the Spawn cast).
func TestHandOfEmrakulLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "handofemrakul", khandofemrakulDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
					abs[j].Static.Kind = kop.StatNone
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the alternative cost dropped")
	}
}

// TestHandOfEmrakulLockstepAnnihilatorMutation: with Hand of Emrakul's
// attack trigger dropped, some seed diverges (the pool attacks with it).
func TestHandOfEmrakulLockstepAnnihilatorMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "handofemrakul", khandofemrakulDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Name != "Hand of Emrakul" {
				continue
			}
			var abs []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind != kop.AbTriggered {
					abs = append(abs, a)
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with Annihilator dropped")
	}
}
