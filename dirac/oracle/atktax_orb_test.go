package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The attack tax beside an attack requirement and a non-land mana source, two
// seats (CR 508.1d, 508.1g-i, 106.6; gorge rules/attack_cost.go
// attackManaSources; kernel combat_attacktax.go atkTaxManaAb). The taxer plays
// Propaganda and Archangel of Tithes (it taxes an attack on its controller
// while untapped); the payer plays Carnelian Orb of Dragonkind, whose {T}: Add
// {R} carries a spent-mana haste rider gorge lists as the plain unit it is,
// Juggernaut ("attacks each combat if able": a creature CR 508.1d's last
// sentence frees whenever every pair of it costs mana) and Mountains. The seats
// swap every other pair of seeds, so each deck taxes and pays. Archangel of
// Tithes also carries a block tax, which the kernel refuses beside a MustAttack
// (combat_blocktax.go blkTaxMustAttack), so its scenario holds no Juggernaut.

type atkOrbDecks struct{ taxer, payer []cardset.Entry }

var atkOrbRequirement = atkOrbDecks{
	taxer: []cardset.Entry{{Name: "Propaganda", Count: 4}, {Name: "Serra Angel", Count: 3},
		{Name: "Goblin Piker", Count: 8}, {Name: "Plains", Count: 14}, {Name: "Island", Count: 11}},
	payer: []cardset.Entry{{Name: "Carnelian Orb of Dragonkind", Count: 4}, {Name: "Juggernaut", Count: 4},
		{Name: "Goblin Piker", Count: 8}, {Name: "Propaganda", Count: 1}, {Name: "Mountain", Count: 23}},
}

var atkOrbArchangel = atkOrbDecks{
	taxer: []cardset.Entry{{Name: "Propaganda", Count: 4}, {Name: "Archangel of Tithes", Count: 3}, {Name: "Serra Angel", Count: 3},
		{Name: "Goblin Piker", Count: 6}, {Name: "Plains", Count: 14}, {Name: "Island", Count: 10}},
	payer: []cardset.Entry{{Name: "Carnelian Orb of Dragonkind", Count: 4}, {Name: "Goblin Piker", Count: 12},
		{Name: "Propaganda", Count: 1}, {Name: "Mountain", Count: 23}},
}

// runAtkOrb plays seeds two-seat games of the two decks in lockstep with mut
// set to on for the run, and returns the games played, the divergences and the
// tally of what the games reached.
func runAtkOrb(t *testing.T, dk atkOrbDecks, seeds uint64, mut *bool, on bool) (games, divs int, tally map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, ents := range [][]cardset.Entry{dk.taxer, dk.payer} {
		for _, e := range ents {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			pool = append(pool, c)
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "atktax-orb-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	old := *mut
	*mut = on
	defer func() { *mut = old }()
	tally = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		ents := [2][]cardset.Entry{dk.taxer, dk.payer}
		if (seed/2)%2 == 1 {
			ents[0], ents[1] = ents[1], ents[0]
		}
		var decks [2][]*cards.Card
		for s := range decks {
			for _, e := range ents[s] {
				c, _ := reg.Lookup(e.Name)
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
			t.Fatalf("the kernel refuses the matchup: %v", err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"orb-a", "orb-b"}, decks, 1)
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
					if len(d.Options) > 0 && d.Options[0].Kind == "attack_mana" {
						tally["window taps"]++
						if o := optByIndex(d, idx); o.Obj != 0 {
							if ob := gs.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
								tally["tap "+ob.Face().Name]++
							}
						}
					}
				}
			}
			if c.G != nil && c.G.fr != nil && c.G.fr.d != nil && c.G.fr.d.Kind == decision.KAttackers {
				for _, o := range c.G.fr.d.Options {
					if o.Required {
						tally["required attack offers"]++
					}
				}
			}
			a := inner.Pick(c)
			if a.Kind() == ActAttack && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Juggernaut" {
						tally["Juggernaut attacks"]++
					}
				}
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if on {
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

// TestAtkTaxOrbLockstep: the Orb pays an attack tax beside a requirement
// (Propaganda, Juggernaut) and beside Archangel of Tithes's tax, gorge and dirac
// decision by decision.
func TestAtkTaxOrbLockstep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		dk   atkOrbDecks
		min  map[string]int
	}{
		{"requirement", atkOrbRequirement, map[string]int{"window taps": 10, "tap Carnelian Orb of Dragonkind": 3,
			"required attack offers": 3, "Juggernaut attacks": 2}},
		{"archangel", atkOrbArchangel, map[string]int{"window taps": 10, "tap Carnelian Orb of Dragonkind": 3}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var off bool
			games, divs, tally := runAtkOrb(t, c.dk, 120, &off, false)
			t.Logf("%d games, %d divergences; %s", games, divs, katTally(tally))
			// PRECONDITION: the games reached the shape, or no divergence proves nothing.
			for key, min := range c.min {
				if tally[key] < min {
					t.Errorf("%s %d, want at least %d", key, tally[key], min)
				}
			}
		})
	}
}

// TestAtkTaxOrbFreeMutation: a kernel that taxes no attack diverges on the same
// games, so the lockstep above reads the tax.
func TestAtkTaxOrbFreeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, dk := range map[string]atkOrbDecks{"requirement": atkOrbRequirement, "archangel": atkOrbArchangel} {
		games, divs, _ := runAtkOrb(t, dk, 120, &dirac.DebugAtkTaxFree, true)
		if divs == 0 {
			t.Errorf("%s: an untaxed kernel played %d games with no divergence", name, games)
		}
	}
}
