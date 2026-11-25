package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// keurekaDecks is the Eureka lockstep (dirac/kgen/lower_imprintzone.go
// handMoveImprint): each pass clears the spell's imprinted set, each player
// may put a permanent card from their hand onto the battlefield (imprinted),
// and the loop repeats while one was put. A missed or extra imprint changes
// the number of passes, so the next pass's pick or the end of the spell
// diverges.
func keurekaDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Eureka", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Ornithopter", Count: 4},
			{Name: "Glorious Anthem", Count: 4}, {Name: "Forest", Count: 16},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Hypergenesis", Count: 4},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		},
	}
}

// keurekaPolicy is castFirstPolicy counting Eureka's casts.
func keurekaPolicy(casts *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if c.G != nil && c.D.Kind == DPriority && a.Kind() == ActCast {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Eureka" {
						casts.Add(1)
					}
				}
			}
			return a
		})
	}
}

// TestEurekaLockstep: Eureka's games agree with gorge.
func TestEurekaLockstep(t *testing.T) {
	t.Parallel()
	var casts atomic.Int64
	lockstepRunPolicy(t, "eureka", keurekaDecks(), 200, true, nil, nil, keurekaPolicy(&casts))
	if casts.Load() == 0 {
		t.Fatal("Eureka was never cast; the lockstep proves nothing")
	}
	t.Logf("Eureka casts: %d", casts.Load())
}

// TestEurekaMutation proves the imprint bites: with the hand pick's imprint
// dropped, the loop ends after one pass and some seed diverges.
func TestEurekaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugHandPickNoImprint = true
	defer func() { dirac.DebugHandPickNoImprint = false }()
	var casts atomic.Int64
	if lockstepRunPolicy(t, "eureka", keurekaDecks(), 200, false, nil, nil, keurekaPolicy(&casts)) == 0 {
		t.Fatal("no seed diverged with the hand pick's imprint dropped")
	}
}
