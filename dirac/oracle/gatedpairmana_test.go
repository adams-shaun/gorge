package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A gated same-colour larger mana ability (Whisperer of the Wilds: {T}: Add
// {G}, then a gated {T}: Add {G}{G}): while the gate holds gorge lists the
// creature as one unit with both alternatives and takes the one that ranks best
// per payment, and the payment window answers ability 0. The kernel's unit is
// the restricted source's pair without the restricted float (mana_gate.go).
// The deck pays one-, two- and three-green costs with the gate on and Forests
// beside the creature.
var gatedPairDeck = []cardset.Entry{
	{Name: "Whisperer of the Wilds", Count: 5}, {Name: "Leatherback Baloth", Count: 6},
	{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 6}, {Name: "Forest", Count: 17},
}

// gatedPairNames are the spells whose payments the probe tallies: the cost in
// green mana each is paid with.
var gatedPairNames = map[string]string{"Giant Growth": "G", "Grizzly Bears": "1G", "Leatherback Baloth": "GGG"}

type gatedPairStats struct{ paid map[string]*atomic.Int64 }

func newGatedPairStats() *gatedPairStats {
	s := &gatedPairStats{paid: map[string]*atomic.Int64{}}
	for n := range gatedPairNames {
		s.paid[n] = new(atomic.Int64)
	}
	return s
}

// policy casts eagerly and tallies each cast of a tallied spell made with the
// gate on: an untapped Whisperer beside an untapped Forest, and a creature of
// power four or more, all the caster's (gorge leads the decision).
func (s *gatedPairStats) policy(seed uint64) Policy {
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
		if ob == nil || ob.Face() == nil || gatedPairNames[ob.Face().Name] == "" {
			return a
		}
		p := state.PlayerID(c.D.Player)
		whisp, forest, big := false, false, false
		for _, id := range g.Zone(state.ZBattlefield, p) {
			b := g.Obj(id)
			if b == nil || b.Face() == nil || b.Controller != p {
				continue
			}
			switch {
			case b.Face().Name == "Whisperer of the Wilds" && !b.Tapped:
				whisp = true
			case b.Face().Name == "Forest" && !b.Tapped:
				forest = true
			}
			big = big || c.G.E.Power(id) >= 4
		}
		if whisp && forest && big {
			s.paid[ob.Face().Name].Add(1)
		}
		return a
	})
}

func gatedPairDecks() [2][]cardset.Entry { return [2][]cardset.Entry{gatedPairDeck, gatedPairDeck} }

// gatedPairMinPaid is how many gate-on payments of each cost the run must make.
const gatedPairMinPaid = 3

func TestWhispererGatedPairLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := newGatedPairStats()
	lockstepRunPolicy(t, "gatedpair", gatedPairDecks(), 120, true, nil, nil, st.policy)
	for n, c := range st.paid {
		t.Logf("%s (%s): %d payments with the gate on beside a Forest", n, gatedPairNames[n], c.Load())
		if c.Load() < gatedPairMinPaid {
			t.Errorf("%s: %d gate-on payments, want at least %d (the pool does not exercise the pair)", n, c.Load(), gatedPairMinPaid)
		}
	}
}

// TestWhispererGatedPairMutation: with the larger alternative left out of the
// gate-on view (1) or the planner's pair arm left out (2) some game diverges
// from gorge, so the pair and the planner arm are what the lockstep checks.
func TestWhispererGatedPairMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mut := range []uint8{1, 2} {
		dirac.SetDebugGatedPair(mut)
		div := lockstepRunPolicy(t, "gatedpair", gatedPairDecks(), 120, false, nil, nil, castFirstPolicy)
		dirac.SetDebugGatedPair(0)
		if div == 0 {
			t.Errorf("mutation %d did not diverge", mut)
		}
	}
}
