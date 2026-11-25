package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted hand cycling lockstep (CR 702.29a, 702.29e): a battlefield
// static granting Cycling or TypeCycling to cards in a hand (Rhet-Tomb
// Mystic's "each creature card in your hand has cycling {1}{U}", Jo Grant's
// historic cards, Tectonic Reformation's lands, Homing Sliver's slivercycling
// for each player's hand) gives the kernel the same hand offer gorge's
// derived-keyword walk gives: the granted body from the hand for the
// grant's own cost, the card discarded as the cost.
//
// The pool is the off-battlefield statics fixture (offzonegrants). The first
// deck runs the Mystic over vanilla creatures; the second runs the other
// three grantors, with the opponent running Homing Sliver too (its grant
// reaches the other player's hand).
var offZoneCyclingDecks = [2][]cardset.Entry{
	{
		{Name: "Rhet-Tomb Mystic", Count: 8}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Island", Count: 18}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Raging Goblin", Count: 10}, {Name: "Hill Giant", Count: 8},
		{Name: "Mountain", Count: 16}, {Name: "Swamp", Count: 6},
	},
}

var offZoneCyclingDecksB = [2][]cardset.Entry{
	{
		{Name: "Homing Sliver", Count: 8}, {Name: "Jo Grant", Count: 6}, {Name: "Tectonic Reformation", Count: 6},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Homing Sliver", Count: 8}, {Name: "Tectonic Reformation", Count: 4}, {Name: "Raging Goblin", Count: 6},
		{Name: "Mountain", Count: 18}, {Name: "Swamp", Count: 4},
	},
}

// offZoneCyclingPolicy is castFirstPolicy biased to the activations: these
// decks' only activated abilities are cycling (the granted one, and Homing
// Sliver's printed slivercycling in the second pair), so any activation
// gorge offers is taken in 3 of 4 picks, otherwise the base policy casts and
// plays lands, so the seeds reach a grantor on the board with a card in hand
// and exercise the discard, the draw and the search.
func offZoneCyclingPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0xc1c11e))
	return PolicyFunc(func(c *PickCtx) Canon {
		var act []Canon
		for _, x := range c.D.Options {
			if x.Kind() == ActActivate {
				act = append(act, x)
			}
		}
		if len(act) > 0 && r.IntN(4) != 0 {
			return act[r.IntN(len(act))]
		}
		return base.Pick(c)
	})
}

func testOffZoneCycling(t *testing.T, decks [2][]cardset.Entry) {
	t.Helper()
	_, set, gd := lockstepPool(t, offZoneUnearthPool, decks)
	if !SetDeckSupported(set, gd[0]) || !SetDeckSupported(set, gd[1]) {
		t.Fatal("the cycling decks are not playable in the offzonegrants pool")
	}
	before := dirac.CyclingGrantStats.Activations.Load()
	lockstepRunPolicy(t, offZoneUnearthPool, decks, 64, true, nil, nil, offZoneCyclingPolicy)
	if n := dirac.CyclingGrantStats.Activations.Load() - before; n == 0 {
		t.Fatal("no granted cycling activation ran; the lockstep proves nothing")
	} else {
		t.Logf("granted cycling activations: %d", n)
	}
}

func TestOffZoneCyclingLockstep(t *testing.T) {
	testOffZoneCycling(t, offZoneCyclingDecks)
}

func TestOffZoneCyclingLockstepHandGrants(t *testing.T) {
	testOffZoneCycling(t, offZoneCyclingDecksB)
}
