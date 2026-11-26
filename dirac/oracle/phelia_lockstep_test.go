package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kpheliaDecks is the Phelia, Exuberant Shepherd lockstep
// (dirac/kgen/lower_imprintzone.go returnImprint): her attack exiles up to
// one other nonland permanent, the end step returns it imprinted, and she
// gets a +1/+1 counter when the imprinted card entered under her
// controller's control. The kernel imprints the object before its Return
// and forgets it when the Return did not move it (a Pacifism with nothing
// to enchant stays in exile). A missed or extra imprint moves the counter.
func kpheliaDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Phelia, Exuberant Shepherd", Count: 10}, {Name: "Savannah Lions", Count: 8},
			{Name: "Pacifism", Count: 6}, {Name: "Plains", Count: 16},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 12},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
		},
	}
}

// kpheliaPolicy is castFirstPolicy counting Phelia's attack triggers'
// targets: each time a target is chosen for her trigger.
func kpheliaPolicy(casts *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if c.G != nil && c.D.Kind == DPriority && a.Kind() == ActCast {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Phelia, Exuberant Shepherd" {
						casts.Add(1)
					}
				}
			}
			return a
		})
	}
}

// TestPheliaLockstep: Phelia's games agree with gorge.
func TestPheliaLockstep(t *testing.T) {
	t.Parallel()
	var casts atomic.Int64
	lockstepRunPolicy(t, "phelia", kpheliaDecks(), 200, true, nil, nil, kpheliaPolicy(&casts))
	if casts.Load() == 0 {
		t.Fatal("Phelia was never cast; the lockstep proves nothing")
	}
	t.Logf("Phelia casts: %d", casts.Load())
}

// TestPheliaMutation proves the bracket bites: with the pre-move imprint
// dropped, Phelia never gets her counter and some seed diverges.
func TestPheliaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugImprintRefOff = true
	defer func() { dirac.DebugImprintRefOff = false }()
	var casts atomic.Int64
	if lockstepRunPolicy(t, "phelia", kpheliaDecks(), 200, false, nil, nil, kpheliaPolicy(&casts)) == 0 {
		t.Fatal("no seed diverged with the Return's imprint dropped")
	}
}
