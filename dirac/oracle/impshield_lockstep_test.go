package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kimpshieldDecks is the Dazzling Reflection lockstep
// (dirac/kgen/lower_effectimprintshield.go): the Effect imprints the spell's
// target and prevents the next damage that creature deals, exiling itself
// then or when the creature leaves first; the kernel keeps Awe Strike's
// source-keyed shield (kop.PrevNextFrom) on the target. The life gain equal
// to the target's power and the prevented damage are in the projection, so
// a missed, extra or late prevention diverges.
func kimpshieldDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Dazzling Reflection", Count: 8}, {Name: "Savannah Lions", Count: 12},
			{Name: "Plains", Count: 20},
		},
		kprevscopeDecks[1],
	}
}

// kimpshieldPolicy casts a Dazzling Reflection the first time gorge offers
// one in a turn and never a second one that turn, and counts the casts. Two
// shields matching one event make gorge ask the CR 616.1 order, which the
// kernel stalls (StallPrevNext, repl_prevnext.go), as it does for Awe
// Strike's.
func kimpshieldPolicy(casts *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		last := int32(-1)
		var buf []Canon
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil || c.D.Kind != DPriority {
				return inner.Pick(c)
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Dazzling Reflection" {
						if c.G.E.G.Turn != last {
							last = c.G.E.G.Turn
							casts.Add(1)
							return a
						}
						continue
					}
				}
				buf = append(buf, a)
			}
			d := *c.D
			d.Options = buf
			return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

// TestImpShieldLockstep: Dazzling Reflection's games agree with gorge.
func TestImpShieldLockstep(t *testing.T) {
	t.Parallel()
	var casts atomic.Int64
	lockstepRunPolicy(t, "impshield", kimpshieldDecks(), 300, true, nil, nil, kimpshieldPolicy(&casts))
	if casts.Load() == 0 {
		t.Fatal("Dazzling Reflection was never cast; the lockstep proves nothing")
	}
	t.Logf("Dazzling Reflection casts: %d", casts.Load())
}

// TestImpShieldMutation proves the shield bites: with every PreventNext
// shield installed but preventing nothing, some seed diverges.
func TestImpShieldMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevNext = true
	defer func() { dirac.DebugNoPrevNext = false }()
	var casts atomic.Int64
	if lockstepRunPolicy(t, "impshield", kimpshieldDecks(), 300, false, nil, nil, kimpshieldPolicy(&casts)) == 0 {
		t.Fatal("no seed diverged with the shield disabled: the pool never has damage prevented by Dazzling Reflection")
	}
}
