package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Azra Oddsmaker ("at the beginning of combat on your turn, you may discard a
// card. If you do, choose a creature. Whenever that creature deals combat
// damage to a player this turn, you draw two cards") in lockstep with gorge.
// The Effect has no RememberObjects$ and its trigger reads
// ValidSource$ Card.ChosenCardStrict; gorge keeps the creature chosen at the
// Effect past the chain's ClearChosenCard$ cleanup (engine/rules
// TestAzraOddsmakerEffectKeepsChosenCreature), and kgen captures the chosen
// object in the Effect's emblem (Emblem.Capture = RefChosen, lowerCtx
// chosenCardCapture), so only that creature's combat damage draws.

var mbcAzraDecks = [2][]cardset.Entry{
	{
		{Name: "Azra Oddsmaker", Count: 12}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Azra Oddsmaker", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Savannah Lions", Count: 2}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 2},
	},
}

// mbcAzraProbe counts, over all games, the registrations of an Azra
// Oddsmaker Effect's DamageDone trigger gorge held at a pick (ids repeat from
// game to game, so each policy instance, one per seed, remembers its own).
func mbcAzraProbe(base Policy, armed *int) Policy {
	seen := map[uint64]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, dt := range c.G.E.G.Delayed {
				if o := c.G.E.G.Obj(dt.Source); dt.EventMode == "DamageDone" && o != nil && o.Face() != nil && o.Face().Name == "Azra Oddsmaker" && !seen[uint64(dt.ID)] {
					seen[uint64(dt.ID)] = true
					*armed++
				}
			}
		}
		return base.Pick(c)
	})
}

func TestMBCAzraOddsmakerLowers(t *testing.T) {
	t.Parallel()
	armed := 0
	pol := func(seed uint64) Policy { return mbcAzraProbe(castFirstPolicy(seed), &armed) }
	const seeds = 160
	if div := lockstepRunPolicy(t, "mbcazra", mbcAzraDecks, seeds, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("%d Azra Oddsmaker Effect registrations armed over %d seeds", armed, seeds)
	if armed < 20 {
		t.Fatalf("%d Azra Oddsmaker Effect registrations over %d seeds, want at least 20: the pool does not exercise it", armed, seeds)
	}
}

// TestMBCAzraOddsmakerCaptureMutation: an emblem that captures nothing fires
// for any creature's combat damage, where gorge draws only for the chosen one.
func TestMBCAzraOddsmakerCaptureMutation(t *testing.T) {
	t.Parallel()
	drop := func(tbl *kop.Table) {
		tbl.Emblems = append([]kop.Emblem(nil), tbl.Emblems...)
		for i := range tbl.Emblems {
			tbl.Emblems[i].Capture = kop.RefNone
		}
	}
	armed := 0
	pol := func(seed uint64) Policy { return mbcAzraProbe(castFirstPolicy(seed), &armed) }
	if div := lockstepRunPolicy(t, "mbcazra", mbcAzraDecks, 160, false, drop, nil, pol); div == 0 {
		t.Fatal("an Azra Oddsmaker emblem with no capture played 160 seeds with no divergence")
	}
}
