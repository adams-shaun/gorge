package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kparallaxDecks is the Parallax Nexus lockstep (dirac/kgen/lower_imprintzone.go
// handMoveImprint on the persist shape): each activation makes the target
// opponent exile a card from their hand, imprinted on the Nexus's kept list
// (zone_imprint.go); when the Nexus leaves (Fading's sacrifice), its leave
// trigger returns every imprinted card to its owner's hand. A missed record
// leaves a card in exile, a wrong one moves another card.
func kparallaxDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Parallax Nexus", Count: 12}, {Name: "Walking Corpse", Count: 12}, {Name: "Swamp", Count: 16},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Grizzly Bears", Count: 10},
			{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		},
	}
}

// kparallaxPolicy is castFirstPolicy counting the Nexus's activations.
func kparallaxPolicy(acts *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if c.G != nil && c.D.Kind == DPriority && a.Kind() == ActActivate {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Parallax Nexus" {
						acts.Add(1)
					}
				}
			}
			return a
		})
	}
}

// TestParallaxLockstep: Parallax Nexus's games agree with gorge.
func TestParallaxLockstep(t *testing.T) {
	t.Parallel()
	var acts atomic.Int64
	lockstepRunPolicy(t, "parallax", kparallaxDecks(), 200, true, nil, nil, kparallaxPolicy(&acts))
	if acts.Load() == 0 {
		t.Fatal("Parallax Nexus was never activated; the lockstep proves nothing")
	}
	t.Logf("Parallax Nexus activations: %d", acts.Load())
}

// TestParallaxMutation proves the kept list bites: with the source's
// Imprinted records dropped, the leave trigger returns nothing and some seed
// diverges.
func TestParallaxMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSrcMemNoImp = true
	defer func() { dirac.DebugSrcMemNoImp = false }()
	var acts atomic.Int64
	if lockstepRunPolicy(t, "parallax", kparallaxDecks(), 200, false, nil, nil, kparallaxPolicy(&acts)) == 0 {
		t.Fatal("no seed diverged with the Nexus's Imprinted list dropped")
	}
}
