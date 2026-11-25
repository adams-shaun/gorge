package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// keyesingDecks is the Eye of Singularity lockstep (dirac/kgen/
// lower_imprintzone.go imprintNameSweep): the enters trigger imprints each
// permanent sharing a name with another nonbasic permanent and destroys the
// imprinted set; the kernel destroys Permanent.sharesNameWith Valid
// Permanent.nonBasic in one sweep. Both sides play duplicate creatures and
// Anthems, so a missed or extra destroy diverges.
func keyesingDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Eye of Singularity", Count: 10}, {Name: "Savannah Lions", Count: 8},
			{Name: "Glorious Anthem", Count: 4}, {Name: "Plains", Count: 18},
		},
		{
			{Name: "Grizzly Bears", Count: 14}, {Name: "Savannah Lions", Count: 4}, {Name: "Glorious Anthem", Count: 4},
			{Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
		},
	}
}

// keyesingPolicy is castFirstPolicy counting the Eye's casts.
func keyesingPolicy(casts *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if c.G != nil && c.D.Kind == DPriority && a.Kind() == ActCast {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Eye of Singularity" {
						casts.Add(1)
					}
				}
			}
			return a
		})
	}
}

// TestEyeSingularityLockstep: Eye of Singularity's games agree with gorge.
func TestEyeSingularityLockstep(t *testing.T) {
	t.Parallel()
	var casts atomic.Int64
	lockstepRunPolicy(t, "eyesing", keyesingDecks(), 200, true, nil, nil, keyesingPolicy(&casts))
	if casts.Load() == 0 {
		t.Fatal("Eye of Singularity was never cast; the lockstep proves nothing")
	}
	t.Logf("Eye of Singularity casts: %d", casts.Load())
}
