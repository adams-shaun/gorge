package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The can't-attack-or-block-alone lockstep: Bonded Horncrest's compound
// keyword lowers to CardDef.AloneAtk and CardDef.AloneBlk, and Craven Hulk's
// block-only sibling to AloneBlk. The policy attacks and blocks whenever it
// can, preferring a carrier, so both declarations reach gorge's
// Decision.Validate (combat.DeclaresAlone) and the kernel's matching
// offer/Done fence in combat_alone.go.

var kaloneblockDecks = [2][]cardset.Entry{
	{
		{Name: "Bonded Horncrest", Count: 4}, {Name: "Craven Hulk", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Bonded Horncrest", Count: 4}, {Name: "Craven Hulk", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

// kaloneblockJointDecks adds attackers whose block limits use up a
// carrier's partner: Viashino Runner (two or more blockers) and Safewright
// Cavalry (no more than one); and whose block requirements meet CR 506.5:
// Vinebred Brawler (must be blocked if able) and Prized Unicorn (lure).
var kaloneblockJointDecks = [2][]cardset.Entry{
	{
		{Name: "Bonded Horncrest", Count: 4}, {Name: "Craven Hulk", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Viashino Runner", Count: 3}, {Name: "Safewright Cavalry", Count: 3},
		{Name: "Vinebred Brawler", Count: 3}, {Name: "Prized Unicorn", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Bonded Horncrest", Count: 4}, {Name: "Craven Hulk", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Viashino Runner", Count: 3}, {Name: "Safewright Cavalry", Count: 3},
		{Name: "Vinebred Brawler", Count: 3}, {Name: "Prized Unicorn", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
	},
}

var kaloneblockCarriers = map[string]bool{"Bonded Horncrest": true, "Craven Hulk": true}

// kaloneblockLimited are the attackers whose block limits or requirements
// interact with a carrier's partner.
var kaloneblockLimited = map[string]bool{"Viashino Runner": true, "Safewright Cavalry": true, "Vinebred Brawler": true, "Prized Unicorn": true}

type kaloneblockStats struct {
	games, divs, stalls           int
	carrierAttacks, carrierBlocks int
	limitedBlocks                 int // carrier blocks on a kaloneblockLimited attacker
}

// kaloneblockPolicy attacks and blocks whenever it can, preferring a carrier;
// plays lands and casts otherwise.
func kaloneblockPolicy(seed uint64, st *kaloneblockStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
			want := ActAttack
			if c.D.Kind == DBlockers {
				want = ActBlock
			}
			carrier, other := -1, -1
			for i := range c.D.Options {
				o := &c.D.Options[i]
				if o.Kind() != want {
					continue
				}
				if c.G != nil && kaloneblockCarriers[gorgeOptionName(c, *o)] {
					if carrier < 0 || want == ActBlock && kaloneblockLimited[gorgeAttackerName(c, *o)] {
						carrier = i
					}
				} else if other < 0 {
					other = i
				}
			}
			switch {
			case carrier >= 0:
				if want == ActAttack {
					st.carrierAttacks++
				} else {
					st.carrierBlocks++
					if kaloneblockLimited[gorgeAttackerName(c, c.D.Options[carrier])] {
						st.limitedBlocks++
					}
				}
				return c.D.Options[carrier]
			case other >= 0:
				return c.D.Options[other]
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand || o.Kind() == ActCast {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// gorgeAttackerName is the name of the attacker a canonical block option
// names.
func gorgeAttackerName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok || o.Attacker == 0 {
		return ""
	}
	if ob := c.G.E.G.Obj(o.Attacker); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// runAloneBlock plays seeds of the constructed matchup. mutate sets
// DebugAloneBlkOff (the mutation must diverge).
func runAloneBlock(t *testing.T, decksIn [2][]cardset.Entry, seeds int, mutate bool) *kaloneblockStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("aloneblock")
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the table carries the lowered keyword on both sides, so
	// the lockstep below reads the alone keywords (AloneAtk, AloneBlk), not a plain creature.
	for _, name := range []string{"Bonded Horncrest", "Craven Hulk"} {
		i := slices.Index(set.Names(), name)
		if i < 0 {
			t.Fatalf("%s is not in the aloneblock table", name)
		}
		if def := set.Def(i); !def.AloneBlk || name == "Bonded Horncrest" && !def.AloneAtk {
			t.Fatalf("%s CardDef = AloneAtk %v / AloneBlk %v, want the lowered restriction", name, def.AloneAtk, def.AloneBlk)
		}
	}
	old := dirac.DebugAloneBlkOff
	dirac.DebugAloneBlkOff = mutate
	defer func() { dirac.DebugAloneBlkOff = old }()
	st := &kaloneblockStats{}
	var decks [2][]*cards.Card
	for s, d := range decksIn {
		for _, e := range d {
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
	tbl, _, _, err := set.Matchup(decksIn[0], decksIn[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"aloneblock-a", "aloneblock-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kaloneblockPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

// TestAloneBlockLockstep plays Bonded Horncrest and Craven Hulk through both
// engines, attacking and blocking with them wherever a partner lets them.
func TestAloneBlockLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAloneBlock(t, kaloneblockDecks, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, carrier attacks %d, carrier blocks %d",
		st.games, st.divs, st.stalls, st.carrierAttacks, st.carrierBlocks)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.carrierAttacks < 20 {
		t.Errorf("only %d carrier attacks", st.carrierAttacks)
	}
	if st.carrierBlocks < 20 {
		t.Errorf("only %d carrier blocks", st.carrierBlocks)
	}
}

// TestAloneBlockMutation lets a carrier block alone: the games must diverge.
func TestAloneBlockMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAloneBlock(t, kaloneblockDecks, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation let restricted creatures block alone in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}

// TestAloneBlockJointLockstep plays the carriers against Viashino Runner and
// Safewright Cavalry: a carrier's partner must stay legal with the carrier's
// own block counted (a capped attacker takes no second blocker, a menace
// floor needs a blocker the carrier did not use).
func TestAloneBlockJointLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runAloneBlock(t, kaloneblockJointDecks, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, carrier attacks %d, carrier blocks %d, on limited attackers %d",
		st.games, st.divs, st.stalls, st.carrierAttacks, st.carrierBlocks, st.limitedBlocks)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.limitedBlocks < 10 {
		t.Errorf("only %d carrier blocks on a limited attacker", st.limitedBlocks)
	}
}
