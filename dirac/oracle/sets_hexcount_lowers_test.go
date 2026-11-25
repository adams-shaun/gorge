package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSetsHexproofCountLowers: printed "hexproof from monocolored" (General
// Ferrous Rokiric, Sphinx of the Guildpact; K:Hexproof:Card.MonoColor) and
// "hexproof from each color" (Breaker of Creation; Card.nonColorless) lower
// as CardDef.HexColors' kop.HexMono bit and its five colours, in lockstep
// with gorge's hexproofQuality / sourceHasQuality.
//
// Precondition: gorge's games cast Lightning Bolt (monocoloured, so it can't
// target them) while an opponent controls a holder, and Lightning Helix
// (multicoloured) at a monocolored holder. The mutation clears HexColors
// from the three holders and must diverge.
func TestSetsHexproofCountLowers(t *testing.T) {
	t.Parallel()
	t.Run("lockstep", func(t *testing.T) {
		t.Parallel()
		var st setsHexStats
		pol := func(seed uint64) Policy { return setsHexProbe(castFirstPolicy(seed), &st) }
		if div := lockstepRunPolicy(t, "setshexcount", setsHexDecks, 120, true, nil, nil, pol); div != 0 {
			return // each divergence is already a test error
		}
		t.Logf("%+v", st)
		if st.boltBeside < 10 || st.breaker < 3 || st.helixAt < 3 {
			t.Fatalf("%+v: want Lightning Bolt cast beside an opposing holder at least 10 times (3 beside Breaker of Creation) and Lightning Helix at a monocolored holder at least 3: the pool does not exercise it", st)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		t.Parallel()
		mutate := func(tbl *kop.Table) {
			n := 0
			for i := range tbl.Cards {
				if cd := &tbl.Cards[i]; cd.HexColors != 0 {
					cd.HexColors = 0 // the printed hexproof dropped
					n++
				}
			}
			if n != 3 {
				t.Fatalf("%d cards with HexColors, want 3", n)
			}
		}
		if div := lockstepRunPolicy(t, "setshexcount", setsHexDecks, 60, false, mutate, nil, castFirstPolicy); div == 0 {
			t.Fatal("the holders' hexproof dropped from the kernel, yet no divergence")
		}
	})
}

type setsHexStats struct{ boltBeside, breaker, helixAt int }

// setsHexHolder names the three holders and whether their hexproof is from
// monocolored.
var setsHexHolder = map[string]bool{"General Ferrous Rokiric": true, "Sphinx of the Guildpact": true, "Breaker of Creation": false}

// setsHexProbe wraps base and counts, when gorge leads, each Lightning Bolt
// on the stack while an opponent of its controller controls a holder, and
// each Lightning Helix on the stack targeting a monocolored holder.
func setsHexProbe(base Policy, st *setsHexStats) Policy {
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZStack || o.Face() == nil || seen[o.ID] || len(o.Targets) == 0 {
				continue
			}
			switch o.Face().Name {
			case "Lightning Bolt":
				for j := range g.Objs {
					h := &g.Objs[j]
					if _, ok := setsHexHolder[setsHexName(h)]; ok && h.Zone == state.ZBattlefield && h.Controller != o.Controller {
						if !seen[o.ID] {
							st.boltBeside++
						}
						seen[o.ID] = true
						if setsHexName(h) == "Breaker of Creation" {
							st.breaker++
						}
					}
				}
			case "Lightning Helix":
				for _, tg := range o.Targets {
					if h := g.Obj(tg.Obj); !tg.IsPlayer && h != nil && setsHexHolder[setsHexName(h)] {
						seen[o.ID] = true
						st.helixAt++
						break
					}
				}
			}
		}
		return a
	})
}

func setsHexName(o *state.Object) string {
	if o == nil || o.Face() == nil {
		return ""
	}
	return o.Face().Name
}

// setsHexDecks: the holders and Wastes for Breaker of Creation's {C}{C} on
// one side, Bolts and Helixes on the other (and a few on the first, so both
// seats' holders are tried).
var setsHexDecks = [2][]cardset.Entry{
	{
		{Name: "General Ferrous Rokiric", Count: 8}, {Name: "Sphinx of the Guildpact", Count: 4}, {Name: "Breaker of Creation", Count: 3},
		{Name: "Lightning Helix", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 7}, {Name: "Plains", Count: 6}, {Name: "Wastes", Count: 7},
	},
	{
		{Name: "Lightning Bolt", Count: 10}, {Name: "Lightning Helix", Count: 8}, {Name: "Raging Goblin", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 8},
	},
}
