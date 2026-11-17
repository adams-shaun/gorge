package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Defined$ Targeted on a node with no target slot of its own reads the
// parent's slot (dirac/kgen lower_dice.go: FlipCoin declares its own
// ValidTgts$ slot, which the win/lose branches' Pump read; lower_counters.go:
// `Defined$ Self & Targeted` beside the node's own ValidTgts$ declares the slot
// once). Orcish Captain's flip pumps the Orc it targets (+2/+0 or -0/-2);
// Drillworks Mole and Beta Ray Bill put a counter on themselves and on up to
// one target commander / God (none in these games: the empty slot).
func kdefinedTargetedParentDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Orcish Captain", Count: 4},
		{Name: "Drillworks Mole", Count: 4},
		{Name: "Beta Ray Bill", Count: 4},
		{Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10},
		{Name: "Forest", Count: 5},
		{Name: "Plains", Count: 5},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kdefinedTargetedParentPolicy casts, activates and plays lands whenever it
// can, else random, and counts the activations of the carriers' abilities.
func kdefinedTargetedParentPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && !counted[c.Seq] {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob == nil || ob.Face() == nil || ob.Zone != state.ZBattlefield {
						continue
					}
					if *st == nil {
						*st = map[string]int{}
					}
					(*st)[ob.Face().Name]++
					counted[c.Seq] = true
					break
				}
			}
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
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func TestDefinedTargetedParentLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "definedtargetedparent", kdefinedTargetedParentDecks(), 80, true, nil, nil, kdefinedTargetedParentPolicy(st))
	for _, name := range []string{"Orcish Captain", "Drillworks Mole"} {
		if (*st)[name] == 0 {
			t.Errorf("%s's ability was never activated in the games; the lockstep proves nothing (%v)", name, *st)
		}
	}
	t.Logf("activations: %v", *st)
}
