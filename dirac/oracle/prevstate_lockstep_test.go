package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The scoped-prevention-with-a-rider lockstep (dirac/repl_prevx.go): Chant of
// Vitu-Ghazi prevents all damage creatures would deal this turn and its
// controller gains life equal to the damage prevented (gorge
// damageReplacementPreventsWithRider runs the GainLife body for each damage
// event). The expected answer at every decision is gorge's: the life totals
// are in the projection, so a missed or extra gain diverges.
//
// Gorge offers a Convoke cast only when the credit of the untapped creatures
// leaves nothing for the lands to pay (its planner prices no convoke cast, so
// a Chant paid by lands is no play of the canonical list): the first deck is
// mostly Savannah Lions, eight or more of which must be untapped at once.
func kchantDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Chant of Vitu-Ghazi", Count: 8}, {Name: "Savannah Lions", Count: 26},
			{Name: "Plains", Count: 6},
		},
		kprevscopeDecks[1],
	}
}

// kchantPolicy is castFirstPolicy that casts a Chant whenever gorge offers one
// and counts, from gorge's event log, the life gains: neither deck has another source of life gain (no lifelink, no
// gain effect), so each positive life change of the games is a Chant's
// rider run.
func kchantPolicy(gains *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		seen := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.G.E != nil && c.G.E.L != nil {
				evs := c.G.E.L.Events
				for ; seen < len(evs); seen++ {
					if evs[seen].Kind == events.LifeChange && evs[seen].Amount > 0 {
						gains.Add(1)
					}
				}
			}
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Chant of Vitu-Ghazi" {
							return a
						}
					}
				}
			}
			return inner.Pick(c)
		})
	}
}

func TestChantLockstep(t *testing.T) {
	t.Parallel()
	var gains atomic.Int64
	lockstepRunPolicy(t, "prevstate", kchantDecks(), 900, true, nil, nil, kchantPolicy(&gains))
	if gains.Load() == 0 {
		t.Fatal("Chant of Vitu-Ghazi never gained life in the games; the lockstep proves nothing")
	}
	t.Logf("Chant life gains: %d", gains.Load())
}

// TestChantMutation proves the pool gains the prevented life: with the rider
// skipped (the prevention still applying), some seed diverges.
func TestChantMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	var gains atomic.Int64
	if lockstepRunPolicy(t, "prevstate", kchantDecks(), 900, false, nil, nil, kchantPolicy(&gains)) == 0 {
		t.Fatal("no seed diverged with the rider skipped: the pool never has damage prevented by Chant")
	}
}
