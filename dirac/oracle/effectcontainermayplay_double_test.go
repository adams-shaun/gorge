package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two Mission Briefings resolving in one turn can choose the same graveyard
// card: two live Effects then each grant its cast and register their own
// stack-to-graveyard exile replacement over it, and the kernel marks the one
// card's permission records twice. The policy casts a carrier from hand
// before any granted graveyard cast and takes the first card offered, so the
// second Briefing resolves while the first one's permission is unspent and
// chooses the same card (dozens of games over the seeds; a cast of that card
// under both Effects is the kernel's stall, TestStackExileTwoEffectsOneCard).
var effectContainerMayPlayDoubleDecks = [2][]cardset.Entry{
	{
		{Name: "Mission Briefing", Count: 16}, {Name: "Island", Count: 24},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// effectContainerMayPlayCarrierFirst takes the first two hand casts of card
// each turn ahead of everything else the inner policy would pick, so the
// second resolves while the first one's graveyard permission is unspent and
// the mana left is for casting the chosen card.
func effectContainerMayPlayCarrierFirst(card string, st *effectContainerMayPlayStats, inner func(uint64) Policy) func(uint64) Policy {
	return func(seed uint64) Policy {
		in := inner(seed)
		var turn int32
		cast := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.D.Kind == DChoose && len(c.D.Options) > 0 {
				return c.D.Options[0] // both Briefings pick the same graveyard card
			}
			if c.D.Kind == DPriority && c.G != nil {
				if t := c.G.E.G.Turn; t != turn {
					turn, cast = t, 0
				}
				for _, a := range c.D.Options {
					if a.Kind() != ActCast {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || effectContainerMayPlayName(c.G, o.Obj) != card {
						continue
					}
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZHand && cast < 2 {
						cast++
						st.carrierCasts++
						return a
					}
				}
			}
			return in.Pick(c)
		})
	}
}

func TestEffectContainerMayPlayDoubleGrantLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, effectContainerMayPlayPool, effectContainerMayPlayDoubleDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("the double-grant deck is not playable in the pool")
	}
	var st effectContainerMayPlayStats
	var inner effectContainerMayPlayStats
	clean, casts, div := lockstepSeeds(t, reg, set, effectContainerMayPlayPool, effectContainerMayPlayDoubleDecks, gd, 1, 200, true, nil, nil,
		effectContainerMayPlayCarrierFirst("Mission Briefing", &st, effectContainerMayPlayPolicy("Mission Briefing", "", &inner)))
	if div > 0 {
		t.Fatalf("%d of %d games diverged", div, clean+div)
	}
	if st.carrierCasts == 0 || inner.zoneCasts == 0 {
		t.Fatalf("the scenario never ran (%d carrier casts, %d graveyard casts over %d casts/activations)",
			st.carrierCasts, inner.zoneCasts, casts)
	}
	t.Logf("%d clean games, %d carrier casts, %d graveyard casts", clean, st.carrierCasts, inner.zoneCasts)
}
