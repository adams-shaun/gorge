package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The divided-counters lockstep: "distribute N +1/+1
// counters among one to N targets" (CR 601.2d, 608.2b). Shock answers a
// carrier's trigger so a target is illegal at resolution and loses its share.

var kdivcountersnDecks = [2][]cardset.Entry{
	{
		{Name: "Biogenic Upgrade", Count: 4}, {Name: "Glint Weaver", Count: 4}, {Name: "Armament Dragon", Count: 4},
		{Name: "The Earth Crystal", Count: 4}, {Name: "Case of the Trampled Garden", Count: 4}, {Name: "Revival of the Ancestors", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Hill Giant", Count: 12}, {Name: "Cloudspire Skycycle", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 6},
	},
}

type kdivcountersnStats struct {
	games, divs, multi, illegal int
}

// kdivcountersnCheck counts stack objects with two or more targets and those
// among them that lose a target to a zone change before resolving.
func kdivcountersnCheck(st *kdivcountersnStats) func(*GorgeSide, Side) *Divergence {
	seen := map[state.ObjID]bool{}
	bad := map[state.ObjID]bool{}
	return func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for _, id := range g.Stack {
			o := g.Obj(id)
			if o == nil || len(o.Targets) < 2 {
				continue
			}
			if !seen[id] {
				seen[id] = true
				st.multi++
			}
			if !bad[id] {
				for _, tg := range o.Targets {
					if to := g.Obj(tg.Obj); !tg.IsPlayer && (to == nil || to.Zone != state.ZBattlefield) {
						bad[id] = true
						st.illegal++
						break
					}
				}
			}
		}
		return nil
	}
}

func runDivcountersn(t *testing.T, seeds int, mutate bool) kdivcountersnStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("divcountersn")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugDivNOneEach
	dirac.DebugDivNOneEach = mutate
	defer func() { dirac.DebugDivNOneEach = old }()
	var decks [2][]*cards.Card
	for s, d := range kdivcountersnDecks {
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
	tbl, _, _, err := set.Matchup(kdivcountersnDecks[0], kdivcountersnDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st kdivcountersnStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdivcountersn-a", "kdivcountersn-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := kdivcountersnPolicy(seed)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: kdivcountersnCheck(&st)})
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return st
}

// kdivcountersnMultiOnStack reports a stack object with two or more targets.
func kdivcountersnMultiOnStack(g *state.Game) bool {
	for _, id := range g.Stack {
		if o := g.Obj(id); o != nil && len(o.Targets) >= 2 {
			return true
		}
	}
	return false
}

// kdivcountersnPolicy casts or activates whenever it can, else random.
func kdivcountersnPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf, shock, drop []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf, shock, drop = buf[:0], shock[:0], drop[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a // mana up for Shock
			}
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				// Shock waits for a response to a multi-target object, so it kills
				// a recipient before the division resolves
				if k == ActCast && c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
							if ob.Face().Name != "Shock" {
								// the Shock seat holds its mana for a response
								if seed%2 == 1 && c.D.Player == 1 {
									drop = append(drop, a)
									continue
								}
							} else if !kdivcountersnMultiOnStack(c.G.E.G) {
								drop = append(drop, a)
								continue
							} else {
								shock = append(shock, a)
							}
						}
					}
				}
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 && c.G != nil {
			// a target pick aims at an object a pending divided stack
			// object already targets, so a Shock kills one of its recipients
			g := c.G.E.G
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				for _, id := range g.Stack {
					if so := g.Obj(id); so != nil && len(so.Targets) >= 1 {
						for _, tg := range so.Targets {
							if !tg.IsPlayer && tg.Obj == o.Obj {
								buf = append(buf, a)
							}
						}
					}
				}
			}
		}
		if len(buf) == 0 {
			// take every divided target on offer rather than finishing early
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			if len(drop) == 0 {
				return rnd.Pick(c)
			}
			// the held casts are not on offer
			for _, a := range c.D.Options {
				if !slices.Contains(drop, a) {
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 {
				return rnd.Pick(c)
			}
		}
		d := *c.D
		d.Options = buf
		if len(shock) > 0 {
			d.Options = shock
		}
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestDivcountersnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDivcountersn(t, 256, false)
	t.Logf("%d games, %d divergences, %d multi-target resolutions, %d with a target illegal before resolution", st.games, st.divs, st.multi, st.illegal)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	if st.multi < 40 {
		t.Errorf("only %d resolutions with two or more targets", st.multi)
	}
	if st.illegal < 3 {
		t.Errorf("only %d resolutions with a target illegal before resolving", st.illegal)
	}
}

// TestDivcountersnMutation gives one counter per target instead of N: the
// announced division is wrong, so the games must diverge.
func TestDivcountersnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDivcountersn(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation ran %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
