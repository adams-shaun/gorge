package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The crew alternative cost lockstep: Heart of Kiran's "You may remove a
// loyalty counter from a planeswalker you control rather than pay this
// Vehicle's crew cost" (gorge abilityAlternateCostFor: an AltCostIndex 1
// "ability" option beside the crew ability; kernel kop.Ability.AltOf, the
// canon's AltActivateBase code), between a white deck holding the Vehicle
// and two Ajani planeswalkers and a green creature deck. The alternative is
// offered beside the tap-power crew while a planeswalker holds a loyalty
// counter, and alone when the crew cost cannot be paid (a Vehicle beside no
// untapped creature). The expected answer at every decision is gorge's.
var kcrewaltDecks = [2][]cardset.Entry{
	{
		{Name: "Heart of Kiran", Count: 6}, {Name: "Ajani Goldmane", Count: 4}, {Name: "Ajani, Caller of the Pride", Count: 4},
		{Name: "Savannah Lions", Count: 6}, {Name: "Plains", Count: 20},
	},
	{
		{Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 20},
	},
}

func TestCrewAltCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "crewalt", kcrewaltDecks, 1000, "KCREWALT_SEEDS")
}

// TestCrewAltCostDecksPlayable names a refused Heart of Kiran here, not only by
// a lockstep that never reaches the card.
func TestCrewAltCostDecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, "crewalt", kcrewaltDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the crewalt pool", s)
		}
	}
}

// TestCrewAltCostMutation: with the alternative's loyalty cost made
// unpayable (the kernel never offers the second activation) some seed
// diverges, so the lockstep above does reach a Vehicle crewed by removing a
// loyalty counter beside a planeswalker in play.
func TestCrewAltCostMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "crewalt", kcrewaltDecks, 120, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].AltOf != 0 {
					abs[j].Cost.N = 99
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Error("no seed diverged: the pool never plays a crew alternative cost")
	}
}
