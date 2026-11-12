package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The block-tax lockstep (S:Mode$ CantBlockUnless with a generic mana Cost$;
// CR 509.1c, 509.1d; gorge rules/attack_cost.go, kernel combat_blocktax.go):
// Archangel of Tithes taxes every creature that blocks it, Qal Sisma Behemoth
// and Myr Prototype (a charge per +1/+1 counter) their own blocks, Hipparion
// its blocks of power-3-or-more attackers, Awesome Presence the blocks of the
// creature it enchants and Cowed by Wisdom the blocks of the enchanted
// creature (a charge per card in its controller's hand). The payers' sources
// cover the block window's shapes: basic lands, a dual's two intrinsic
// abilities, Sol Ring's two mana, the choice-shaped Tranquil Cove, the
// chosen-colour Thriving Isle and Llanowar Elves, a mana creature that blocks
// and still pays with its own mana. The seats swap every other pair of seeds.

var kbtTaxer = []cardset.Entry{
	{Name: "Archangel of Tithes", Count: 2}, {Name: "Qal Sisma Behemoth", Count: 2}, {Name: "Myr Prototype", Count: 2},
	{Name: "Hipparion", Count: 2}, {Name: "Awesome Presence", Count: 2}, {Name: "Cowed by Wisdom", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Mind Stone", Count: 2},
	{Name: "Plains", Count: 12}, {Name: "Island", Count: 1}, {Name: "Mountain", Count: 3}, {Name: "Tranquil Cove", Count: 2}, {Name: "Sol Ring", Count: 2},
}

var kbtPayer = []cardset.Entry{
	{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Serra Angel", Count: 2},
	{Name: "Warden of Geometries", Count: 2}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 1},
	{Name: "Tranquil Cove", Count: 2}, {Name: "Thriving Isle", Count: 2}, {Name: "Tundra", Count: 2},
	{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 3},
}

// kbtTaxes are the lowered carriers each game counts the casts of.
var kbtTaxes = []string{"Archangel of Tithes", "Qal Sisma Behemoth", "Myr Prototype", "Hipparion", "Awesome Presence", "Cowed by Wisdom"}

// kbtScan counts each tax card's casts in gorge's log.
func kbtScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.PutOnStack || ev.From != state.ZHand {
			continue
		}
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
			for _, name := range kbtTaxes {
				if ob.Face().Name == name {
					n["cast "+name]++
				}
			}
		}
	}
}

// runBlockTax plays the seeded games; free stops at the first divergence of a
// kernel that taxes no block. The tally counts the charged block picks (gorge's
// option label names the {N}), those of each tax's attacker and each source
// the payment window tapped.
func runBlockTax(t *testing.T, seeds uint64, free bool) (games, divs int, tally map[string]int) {
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.RawByName("blocktax")
	if err != nil {
		t.Fatal(err)
	}
	if free {
		old := dirac.DebugBlkTaxFree
		dirac.DebugBlkTaxFree = true
		defer func() { dirac.DebugBlkTaxFree = old }()
	}
	tally = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		ents := [2][]cardset.Entry{kbtTaxer, kbtPayer}
		if (seed/2)%2 == 1 {
			ents[0], ents[1] = ents[1], ents[0]
		}
		var decks [2][]*cards.Card
		for s := range decks {
			for _, e := range ents[s] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(ents[0], ents[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbt-a", "kbt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		played := map[string]int{}
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && gs == nil {
				gs = c.G
				gs.OnInternal = func(d *decision.Decision, idx int) {
					if len(d.Options) > 0 && d.Options[0].Kind == "block_mana" {
						tally["window taps"]++
						if o := optByIndex(d, idx); o.Obj != 0 {
							if ob := gs.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
								tally["tap "+ob.Face().Name]++
							}
						}
					}
				}
			}
			a := inner.Pick(c)
			if a.Kind() == ActBlock && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok && strings.Contains(o.Label, " (pay {") {
					tally["charged picks"]++
					if ob := c.G.E.G.Obj(o.Attacker); ob != nil && ob.Face() != nil {
						tally["charged block of "+ob.Face().Name]++
					}
				}
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		kbtScan(gs, tally)
		if d := res.Div; d != nil {
			divs++
			if free {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// TestBlockTaxLockstep plays every lowered block tax against a defender whose
// sources span the block window's shapes, gorge and dirac decision by decision.
func TestBlockTaxLockstep(t *testing.T) {
	t.Parallel()
	games, divs, tally := runBlockTax(t, 120, false)
	t.Logf("%d games, %d divergences; %s", games, divs, katTally(tally))
	for _, name := range kbtTaxes {
		if tally["cast "+name] < 2 {
			t.Errorf("%s cast %d times, want at least 2", name, tally["cast "+name])
		}
	}
	for key, min := range map[string]int{"charged picks": 20, "window taps": 20, "charged block of Archangel of Tithes": 2} {
		if tally[key] < min {
			t.Errorf("%s %d, want at least %d", key, tally[key], min)
		}
	}
	if choice := tally["tap Tranquil Cove"] + tally["tap Thriving Isle"]; choice < 1 {
		t.Errorf("the window tapped a choice-shaped source %d times, want at least 1", choice)
	}
}

// TestBlockTaxFreeMutation: a kernel that taxes no block diverges.
func TestBlockTaxFreeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runBlockTax(t, 120, true)
	if divs == 0 {
		t.Fatalf("an untaxed kernel played %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
