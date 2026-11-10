package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A plain {T}: Add {G} beside an evaluated {T}: Add {G} for each creature you
// control (Itlimoc, Cradle of the Sun, the back of Growing Rites of Itlimoc,
// which transforms at the end step with a creature out): gorge lists the land
// as one unit with both alternatives and takes the one that ranks best per
// payment, and the payment window answers ability 0. The kernel's unit is the
// gated pair's, its larger alternative the count (mana_dynpair.go). The deck
// pays one-, two- and three-green costs with Itlimoc out beside two or more
// creatures and Forests.
var dynPairDeck = []cardset.Entry{
	{Name: "Growing Rites of Itlimoc", Count: 6}, {Name: "Leatherback Baloth", Count: 6},
	{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 6}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 4},
}

// dynPairNames are the spells whose payments the probe tallies.
var dynPairNames = map[string]string{"Giant Growth": "G", "Grizzly Bears": "1G", "Leatherback Baloth": "GGG"}

type dynPairStats struct{ paid map[string]*atomic.Int64 }

func newDynPairStats() *dynPairStats {
	s := &dynPairStats{paid: map[string]*atomic.Int64{}}
	for n := range dynPairNames {
		s.paid[n] = new(atomic.Int64)
	}
	return s
}

// policy casts eagerly and tallies each cast of a tallied spell made with an
// untapped Itlimoc and two or more creatures out, all the caster's (gorge
// leads the decision): the count is above one, so the pair has a larger
// alternative.
func (s *dynPairStats) policy(seed uint64) Policy {
	base := eagerPolicy(seed, map[string]int{})
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if a.Kind() != ActCast || c.G == nil {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return a
		}
		g := c.G.E.G
		ob := g.Obj(o.Obj)
		if ob == nil || ob.Face() == nil || dynPairNames[ob.Face().Name] == "" {
			return a
		}
		p := state.PlayerID(c.D.Player)
		land, creatures := false, 0
		for _, id := range g.Zone(state.ZBattlefield, p) {
			b := g.Obj(id)
			if b == nil || b.Face() == nil || b.Controller != p {
				continue
			}
			if b.Face().Name == "Itlimoc, Cradle of the Sun" && !b.Tapped {
				land = true
			}
			if c.G.E.IsCreature(id) {
				creatures++
			}
		}
		if land && creatures >= 2 {
			s.paid[ob.Face().Name].Add(1)
		}
		return a
	})
}

func dynPairDecks() [2][]cardset.Entry { return [2][]cardset.Entry{dynPairDeck, dynPairDeck} }

// dynPairMinPaid is how many such payments of each cost the run must make.
const dynPairMinPaid = 3

func TestItlimocDynPairLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := newDynPairStats()
	lockstepRunPolicy(t, "dynpair", dynPairDecks(), 120, true, nil, nil, st.policy)
	for n, c := range st.paid {
		t.Logf("%s (%s): %d payments with Itlimoc and two creatures out", n, dynPairNames[n], c.Load())
		if c.Load() < dynPairMinPaid {
			t.Errorf("%s: %d payments, want at least %d (the pool does not exercise the pair)", n, c.Load(), dynPairMinPaid)
		}
	}
}

// TestItlimocDynPairMutation: with the planner's pair arm left out
// (SetDebugGatedPair 2) some game diverges from gorge, so the arm is what the
// lockstep checks for the count pair too.
func TestItlimocDynPairMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.SetDebugGatedPair(2)
	div := lockstepRunPolicy(t, "dynpair", dynPairDecks(), 120, false, nil, nil, castFirstPolicy)
	dirac.SetDebugGatedPair(0)
	if div == 0 {
		t.Error("mutation 2 did not diverge")
	}
}
