package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSetsOfferingLowers: Offering (CR 702.48, "you may cast this spell any
// time you could cast an instant by sacrificing a <Type> and paying the
// difference in mana costs"; CM2's Patron of the Nezumi and the corpus' five
// other Offering cards) lowers as the kernel's offering cast
// (kw_offering.go), in lockstep with gorge's offering.go: the instant-speed
// offer, the sacrifice ask before the targets and the symbol-by-symbol
// reduction.
//
// Precondition: gorge's games take the offering cast (counted by the probe).
func TestSetsOfferingLowers(t *testing.T) {
	t.Parallel()
	var casts atomic.Int64
	pol := func(seed uint64) Policy { return setsOfferingProbe(castFirstPolicy(seed), &casts) }
	if div := lockstepRunPolicy(t, "setsoffering", setsOfferingDecks, 80, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("offering casts taken: %d", casts.Load())
	if casts.Load() < 20 {
		t.Fatalf("offering casts taken: %d, want at least 20: the pool does not exercise it", casts.Load())
	}
}

// TestSetsOfferingMutation: the kernel pricing the offering cast at the full
// mana cost must diverge. Not parallel: it sets a kernel debug switch.
func TestSetsOfferingMutation(t *testing.T) {
	dirac.DebugOfferingNoReduce = true
	defer func() { dirac.DebugOfferingNoReduce = false }()
	var casts atomic.Int64
	pol := func(seed uint64) Policy { return setsOfferingProbe(castFirstPolicy(seed), &casts) }
	if div := lockstepRunPolicy(t, "setsoffering", setsOfferingDecks, 40, false, nil, nil, pol); div == 0 {
		t.Fatal("DebugOfferingNoReduce: the offering reduction dropped, yet no divergence")
	}
}

// setsOfferingProbe wraps base: when gorge leads and offers an offering
// cast, it takes it, counting each. While its player holds an Offering card
// and controls a permanent of its quality, it casts Dark Ritual whenever
// offered; it attacks with Savage Ventmaw whenever it can.
func setsOfferingProbe(base Policy, casts *atomic.Int64) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		name := func(x Canon) string {
			if o, ok := c.G.GorgeOption(x); ok {
				if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
					return ob.Face().Name
				}
			}
			if ob := g.Obj(c.G.id.obj[x.Src()]); ob != nil && ob.Face() != nil {
				return ob.Face().Name
			}
			return ""
		}
		if c.D.Kind == DAttackers {
			for _, x := range c.D.Options {
				if x.Kind() == ActAttack && name(x) == "Savage Ventmaw" {
					return x
				}
			}
			return a
		}
		if c.D.Kind != DPriority {
			return a
		}
		var ritual Canon
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok || o.Kind != "cast" {
				continue
			}
			if o.Mode == "offering" {
				casts.Add(1)
				return x
			}
			if name(x) == "Dark Ritual" {
				ritual = x
			}
		}
		if ritual != 0 && setsOfferingPlausible(g, state.PlayerID(c.D.Player)) {
			return ritual
		}
		return a
	})
}

// setsOfferingPlausible reports whether p holds an Offering card and controls
// a permanent of its quality.
func setsOfferingPlausible(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		q, ok := o.Face().KeywordParam("Offering")
		if !ok {
			continue
		}
		for _, bid := range g.Zone(state.ZBattlefield, p) {
			if b := g.Obj(bid); b != nil && b.Controller == p && b.Face() != nil && b.Face().HasTypeWord(q) {
				return true
			}
		}
	}
	return false
}

// setsOfferingDecks: gorge prices an offering cast only on the floating
// pool, so each side floats mana: Dark Ritual for Patron of the Nezumi
// beside its Rats, and Savage Ventmaw's attack trigger (RRRGGG through
// combat) for Patron of the Akki and Patron of the Orochi beside their
// Goblins and Snakes. A coloured pip of the sacrificed permanent reduces a
// matching pip, a generic part only generic.
var setsOfferingDecks = [2][]cardset.Entry{
	{
		{Name: "Patron of the Nezumi", Count: 4}, {Name: "Dark Ritual", Count: 4},
		{Name: "Chittering Rats", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Bog Rats", Count: 4},
		{Name: "Ornithopter", Count: 4},
		{Name: "Swamp", Count: 16},
	},
	{
		{Name: "Savage Ventmaw", Count: 4}, {Name: "Patron of the Akki", Count: 3}, {Name: "Patron of the Orochi", Count: 3},
		{Name: "Akki Raider", Count: 3}, {Name: "Akki Underling", Count: 2}, {Name: "Akki Coalflinger", Count: 2},
		{Name: "Ambush Viper", Count: 3}, {Name: "Ankle Biter", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
	},
}
