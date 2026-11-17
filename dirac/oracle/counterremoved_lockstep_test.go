package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mode$ CounterRemoved / CounterRemovedOnce on the card itself (dirac/kgen
// lower_counterremoved.go; kernel trig_counterremoved.go, kw_suspend.go
// suspendTick): Chandra, Fire Artisan's loyalty-removal damage (her -7 pays
// seven loyalty, the cost's CounterChange) and Watcher of Hours' "whenever you
// remove a time counter while it's exiled, surveil 1" off each upkeep of its
// suspend. The policy suspends Watcher whenever it can and always takes
// Chandra's ultimate once she holds seven, so both triggers run.
func kcounterremovedDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Chandra, Fire Artisan", Count: 6},
		{Name: "Watcher of Hours", Count: 6},
		{Name: "Island", Count: 14},
		{Name: "Mountain", Count: 14},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func kcounterremovedPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil || c.D.Kind != DPriority {
				return rnd.Pick(c)
			}
			var pref []Canon
			for _, a := range c.D.Options {
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Face() == nil {
					continue
				}
				switch {
				case a.Kind() == ActActivate && o.Kind == "ability" && ob.Zone == state.ZBattlefield &&
					ob.Face().Name == "Chandra, Fire Artisan" && ob.Counter("LOYALTY") >= 7:
					pref = append(pref, a)
					if !counted[c.Seq] {
						counted[c.Seq] = true
						if *st == nil {
							*st = map[string]int{}
						}
						(*st)["ultimate offered"]++
					}
				case o.Mode == "suspend" && ob.Zone == state.ZHand && ob.Face().Name == "Watcher of Hours":
					pref = append(pref, a)
					if !counted[c.Seq] {
						counted[c.Seq] = true
						if *st == nil {
							*st = map[string]int{}
						}
						(*st)["suspend offered"]++
					}
				}
			}
			if len(pref) == 0 {
				buf := make([]Canon, 0, 8)
				for _, a := range c.D.Options {
					switch a.Kind() {
					case ActCast, ActCastFace, ActActivate, ActPlayLand:
						buf = append(buf, a)
					}
				}
				if len(buf) == 0 || len(buf) == len(c.D.Options) {
					return rnd.Pick(c)
				}
				pref = buf
			}
			d := *c.D
			d.Options = pref
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func TestCounterRemovedLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "counterremoved", kcounterremovedDecks(), 60, true, nil, nil, kcounterremovedPolicy(st))
	if (*st)["ultimate offered"] == 0 || (*st)["suspend offered"] == 0 {
		t.Fatalf("the games never reached the removals (%v); the lockstep proves nothing", *st)
	}
	t.Logf("removal opportunities: %v", *st)
}
