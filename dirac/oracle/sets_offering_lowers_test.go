package oracle

import (
	"sync"
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
// gorge prices an offering cast only on the floating pool under the oracle's
// surfaces, so each deck floats mana: Dark Ritual and Channel the Suns (Patron of the Nezumi),
// Savage Ventmaw's RRRGGG through combat (Patron of the Akki, Patron of the
// Orochi, Blast-Furnace Hellkite) and Channel the Suns' WUBRG (Patron of the
// Kitsune, Patron of the Moon). Precondition: gorge's games take the
// offering cast of every one of the six cards (counted by the probe). The
// two matchups are two tests, each under the one-minute budget.
func TestSetsOfferingLowers(t *testing.T) {
	t.Parallel()
	setsOfferingRun(t, setsOfferingRG, 300, []string{"Patron of the Nezumi", "Patron of the Akki", "Patron of the Orochi", "Blast-Furnace Hellkite"})
}

// TestSetsOfferingLowersWU is TestSetsOfferingLowers' second matchup: the
// white and blue Patrons beside Channel the Suns.
func TestSetsOfferingLowersWU(t *testing.T) {
	t.Parallel()
	setsOfferingRun(t, setsOfferingWU, 300, []string{"Patron of the Kitsune", "Patron of the Moon"})
}

func setsOfferingRun(t *testing.T, decks [2][]cardset.Entry, seeds uint64, want []string) {
	t.Helper()
	st := &setsOfferingStats{n: map[string]int{}}
	pol := func(seed uint64) Policy { return setsOfferingProbe(castFirstPolicy(seed), st) }
	if div := lockstepRunPolicy(t, "setsoffering", decks, seeds, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("offering casts taken: %v", st.n)
	for _, name := range want {
		if st.n[name] < 2 {
			t.Errorf("%s: %d offering casts, want at least 2: the pool does not exercise it", name, st.n[name])
		}
	}
}

// TestSetsOfferingMutation: the kernel pricing the offering cast at the full
// mana cost must diverge. Not parallel: it sets a kernel debug switch.
func TestSetsOfferingMutation(t *testing.T) {
	dirac.DebugOfferingNoReduce = true
	defer func() { dirac.DebugOfferingNoReduce = false }()
	st := &setsOfferingStats{n: map[string]int{}}
	pol := func(seed uint64) Policy { return setsOfferingProbe(castFirstPolicy(seed), st) }
	if div := lockstepRunPolicy(t, "setsoffering", setsOfferingRG, 200, false, nil, nil, pol); div == 0 {
		t.Fatal("DebugOfferingNoReduce: the offering reduction dropped, yet no divergence")
	}
}

// setsOfferingStats counts the offering casts taken, by card name.
type setsOfferingStats struct {
	mu sync.Mutex
	n  map[string]int
}

// setsOfferingRituals are the pool sources the probe casts while an
// offering is plausible.
var setsOfferingRituals = map[string]bool{"Dark Ritual": true, "Channel the Suns": true}

// setsOfferingProbe wraps base: when gorge leads and offers an offering
// cast, it takes it, counting each. While its player holds an Offering card
// and controls a permanent of its quality, it casts a ritual whenever
// offered; it attacks with Savage Ventmaw whenever it can.
func setsOfferingProbe(base Policy, st *setsOfferingStats) Policy {
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
				st.mu.Lock()
				st.n[name(x)]++
				st.mu.Unlock()
				return x
			}
			if setsOfferingRituals[name(x)] {
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

// setsOfferingBlack: Patron of the Nezumi beside its Rats, Dark Ritual and
// Channel the Suns.
var setsOfferingBlack = []cardset.Entry{
	{Name: "Patron of the Nezumi", Count: 4}, {Name: "Dark Ritual", Count: 2}, {Name: "Channel the Suns", Count: 4},
	{Name: "Chittering Rats", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Bog Rats", Count: 3},
	{Name: "Ornithopter", Count: 3},
	{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 6},
}

// setsOfferingRG: the black deck against Savage Ventmaw, Patron of the Akki,
// Patron of the Orochi and Blast-Furnace Hellkite beside Goblins, Snakes and
// Clockwork Fox (an artifact: generic only). A coloured pip of the
// sacrificed permanent reduces a matching pip.
var setsOfferingRG = [2][]cardset.Entry{
	setsOfferingBlack,
	{
		{Name: "Savage Ventmaw", Count: 4}, {Name: "Patron of the Akki", Count: 3}, {Name: "Patron of the Orochi", Count: 2},
		{Name: "Blast-Furnace Hellkite", Count: 2}, {Name: "Clockwork Fox", Count: 2},
		{Name: "Akki Raider", Count: 3}, {Name: "Akki Underling", Count: 2},
		{Name: "Ambush Viper", Count: 3}, {Name: "Ankle Biter", Count: 2}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
	},
}

// setsOfferingWU: the black deck against Channel the Suns, Patron of the
// Kitsune and Patron of the Moon beside Foxes and Moonfolk (Meloku's {4}{U} leaves
// the Moon only {1}{U}).
var setsOfferingWU = [2][]cardset.Entry{
	setsOfferingBlack,
	{
		{Name: "Channel the Suns", Count: 4}, {Name: "Patron of the Kitsune", Count: 2}, {Name: "Patron of the Moon", Count: 4},
		{Name: "Flutterfox", Count: 3}, {Name: "Floodbringer", Count: 3}, {Name: "Guardians of Oboro", Count: 3},
		{Name: "Meloku the Clouded Mirror", Count: 2}, {Name: "Oboro Breezecaller", Count: 1}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5},
	},
}
