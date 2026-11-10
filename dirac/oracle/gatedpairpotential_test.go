package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A gate-on Whisperer of the Wilds is 3 {G} of potential mana in gorge: its
// potential walk (rules potentialManaFor) admits both listed abilities, {G}
// and {G}{G}, and sums them. Dance of the Tumbleweeds' spree prices each
// mode against that bound (gorge modeCostFeasible) after the printed {1}{G}
// is funded, so with two gate-on Whisperers and no other source (one makes
// the {1}{G}, the other must reach the {3} token mode) the mode is offered
// only by a potential that sums both abilities (3), not the larger alone (2).
var gatedPairPotentialDeck = []cardset.Entry{
	{Name: "Whisperer of the Wilds", Count: 12}, {Name: "Leatherback Baloth", Count: 6},
	{Name: "Dance of the Tumbleweeds", Count: 8}, {Name: "Forest", Count: 14},
}

// potentialCasts counts Dance of the Tumbleweeds casts whose spree modes are
// priced with only gate-on Whisperers to pay them (gateOnAlone).
type potentialCasts struct{ n atomic.Int64 }

// gateOnAlone: p controls two untapped Whisperers that can tap, a creature of
// power four or more (the gate), no untapped Forest and no floating mana.
func gateOnAlone(c *PickCtx) bool {
	g := c.G.E.G
	p := state.PlayerID(c.D.Player)
	whisp, forest, big := 0, false, false
	for _, id := range g.Zone(state.ZBattlefield, p) {
		b := g.Obj(id)
		if b == nil || b.Face() == nil || b.Controller != p {
			continue
		}
		switch {
		case b.Face().Name == "Whisperer of the Wilds" && !b.Tapped && !b.SummonSick:
			whisp++
		case b.Face().Name == "Forest" && !b.Tapped:
			forest = true
		}
		big = big || c.G.E.Power(id) >= 4
	}
	return whisp >= 2 && big && !forest && g.Players[p].Pool.Total() == 0
}

// policy is eagerPolicy holding Dance of the Tumbleweeds back until the gate-on
// Whisperers are the only untapped sources, and counting each such cast.
func (s *potentialCasts) policy(seed uint64) Policy {
	base := eagerPolicy(seed, map[string]int{})
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		alone := gateOnAlone(c)
		isTrash := func(a Canon) bool {
			if a.Kind() != ActCast {
				return false
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				return false
			}
			ob := c.G.E.G.Obj(o.Obj)
			return ob != nil && ob.Face() != nil && ob.Face().Name == "Dance of the Tumbleweeds"
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if alone || !isTrash(a) {
				buf = append(buf, a)
			}
		}
		d := *c.D
		if len(buf) != 0 {
			d.Options = buf
		}
		a := base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		if alone && isTrash(a) {
			s.n.Add(1)
		}
		return a
	})
}

// gatedPairPotentialMin is how many such casts the run must make.
const gatedPairPotentialMin = 3

func TestWhispererGatedPairPotentialLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &potentialCasts{}
	decks := [2][]cardset.Entry{gatedPairPotentialDeck, gatedPairPotentialDeck}
	lockstepRunPolicy(t, "gatedpair", decks, 120, true, nil, nil, st.policy)
	t.Logf("Dance of the Tumbleweeds cast with only gate-on Whisperers untapped: %d", st.n.Load())
	if st.n.Load() < gatedPairPotentialMin {
		t.Errorf("%d casts priced by gate-on Whisperers alone, want at least %d (the pool does not reach the potential bound)", st.n.Load(), gatedPairPotentialMin)
	}
}
