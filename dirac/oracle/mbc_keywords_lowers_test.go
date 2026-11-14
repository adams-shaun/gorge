package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCKeywords1Lowers: the Mystery Booster Commander keywords-1 spelling
// family in lockstep over the mbckeywords1 fixture pool, one subtest per
// spelling.
//
//   - background: K:Choose a Background (CR 702.124k, Ganax, Astral Hunter and
//     Jaheira, Friend of the Forest), a deck-construction marker that lowers to
//     nothing; the pool plays both cards, Jaheira's token mana and Ganax's
//     Dragon-enters Treasure included.
//   - reconfigure: K:Reconfigure (CR 702.151, Rabbit Battery, Bronzeplate Boar,
//     Acquisition Octopus, The Reality Chip): the attach half at another
//     creature you control, the unattach half while attached, and the card
//     being no creature while attached (702.151b), which the lockstep compares
//     through the attack and block options, the statics and the types.
//
// Each subtest's probe reads gorge's event log and fails the test unless every
// card of the spelling was cast and (for reconfigure) attached and unattached,
// so a refused card (the pool would not build) or a game that never reaches
// the mechanism fails as surely as a divergence.
func TestMBCKeywords1Lowers(t *testing.T) {
	t.Parallel()
	t.Run("background", func(t *testing.T) {
		t.Parallel()
		played := map[string]int{}
		pol := func(seed uint64) Policy { return eagerPolicy(seed, played) }
		if div := lockstepRunPolicy(t, mbcKeywords1Pool, mbcBackgroundDecks, 120, true, nil, nil, pol); div != 0 {
			return // each divergence is already a test error
		}
		for _, name := range []string{"Ganax, Astral Hunter", "Jaheira, Friend of the Forest"} {
			t.Logf("%s: cast %d times", name, played[name])
			if played[name] < 20 {
				t.Errorf("%s was cast %d times (want 20) over 120 seeds: the pool does not exercise it", name, played[name])
			}
		}
	})
	t.Run("reconfigure", func(t *testing.T) {
		t.Parallel()
		attach, detach := map[string]int{}, map[string]int{}
		pol := func(seed uint64) Policy { return mbcReconfigureProbe(castFirstPolicy(seed), attach, detach) }
		if div := lockstepRunPolicy(t, mbcKeywords1Pool, mbcReconfigureDecks, 200, true, nil, nil, pol); div != 0 {
			return
		}
		for _, name := range mbcReconfigureCards {
			t.Logf("%s: %d attaches, %d unattaches", name, attach[name], detach[name])
			if attach[name] < 5 || detach[name] < 2 {
				t.Errorf("%s: %d attaches (want 5), %d unattaches (want 2) over 200 seeds: the pool does not exercise it", name, attach[name], detach[name])
			}
		}
	})
}

const mbcKeywords1Pool = "mbckeywords1"

var mbcReconfigureCards = []string{"Rabbit Battery", "Bronzeplate Boar", "Acquisition Octopus", "The Reality Chip"}

// TestMBCKeywords1DecksPlayable asserts each deck of the family is playable in
// the pool's set, so a refused card is named here and not only by a stall far
// into the games.
func TestMBCKeywords1DecksPlayable(t *testing.T) {
	t.Parallel()
	for name, decks := range map[string][2][]cardset.Entry{"background": mbcBackgroundDecks, "reconfigure": mbcReconfigureDecks} {
		_, set, gd := lockstepPool(t, mbcKeywords1Pool, decks)
		for s, d := range gd {
			if !SetDeckSupported(set, d) {
				t.Errorf("%s: seat %d is not playable in the %s pool", name, s, mbcKeywords1Pool)
			}
		}
	}
}

// mbcReconfigureProbe wraps base and tallies, when gorge leads, the Attach
// events its log gains for a Reconfigure card: an attach carries its bearer in
// IDs, the unattach half's Attach event carries none (an SBA detach is a
// different event kind).
func mbcReconfigureProbe(base Policy, attach, detach map[string]int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.Attach {
				continue
			}
			ob := c.G.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil || !ob.Face().HasKeyword("Reconfigure") {
				continue
			}
			if len(ev.IDs) > 0 {
				attach[ob.Face().Name]++
			} else {
				detach[ob.Face().Name]++
			}
		}
		return a
	})
}

// mbcBackgroundDecks: Ganax and bodies on one seat (its Treasures pay Raise the
// Alarm's white), Jaheira and the token makers her mana reads on the other.
// The two never share a deck: a Treasure under Jaheira's grant is a printed
// sacrifice source with a granted tap ability, whose payment order is a
// separate census gap (see the Issues of the ticket's report).
var mbcBackgroundDecks = [2][]cardset.Entry{
	{
		{Name: "Ganax, Astral Hunter", Count: 8}, {Name: "Raise the Alarm", Count: 3},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Plains", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Jaheira, Friend of the Forest", Count: 8}, {Name: "Raise the Alarm", Count: 4},
		{Name: "Dragon Fodder", Count: 4}, {Name: "Plains", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// mbcReconfigureDecks: the four Reconfigure cards, bodies to reconfigure onto
// and for the other side to attack into, the colours to pay for them, and
// Reverse Engineer, whose Improvise ask gorge never offers an attached
// Reconfigure artifact (castasks.go ConvokeAsk).
var mbcReconfigureDecks = [2][]cardset.Entry{
	{
		{Name: "Rabbit Battery", Count: 4}, {Name: "Bronzeplate Boar", Count: 3},
		{Name: "Acquisition Octopus", Count: 3}, {Name: "The Reality Chip", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Reverse Engineer", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Rabbit Battery", Count: 2}, {Name: "Bronzeplate Boar", Count: 2},
		{Name: "Acquisition Octopus", Count: 2}, {Name: "The Reality Chip", Count: 2},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 11}, {Name: "Island", Count: 11},
	},
}
