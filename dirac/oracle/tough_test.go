package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The toughness-assignment lockstep: Bedrock Tortoise
// and Bark of Doran make creatures with power below toughness assign combat
// damage equal to their toughness (CombatDamageToughness), and Interface Ace
// crews and saddles with its toughness (TapPowerValue Value$ Toughness),
// beside power<toughness bodies (Horned Turtle, Giant Spider) and a Vehicle.

var ktoughDecks = [][2][]cardset.Entry{{
	{
		{Name: "Bedrock Tortoise", Count: 4}, {Name: "Bark of Doran", Count: 4}, {Name: "Interface Ace", Count: 4},
		{Name: "Smuggler's Copter", Count: 3}, {Name: "Horned Turtle", Count: 4}, {Name: "Giant Spider", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Savannah Lions", Count: 5},
		{Name: "Glory Seeker", Count: 6}, {Name: "Wall of Omens", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}}

type ktoughStats struct {
	games, divs, stalls int
	toughInCombat       int // attack / block picks of a power<toughness creature under a Tortoise / Bark static
	aceCrews            int // Crew tap picks that tap Interface Ace
}

// ktoughUnderStatic reports whether creature id (gorge's view) is a
// power<toughness creature under its controller's Bedrock Tortoise or a Bark
// of Doran attached to it.
func ktoughUnderStatic(c *PickCtx, id state.ObjID) bool {
	g := c.G.E.G
	o := g.Obj(id)
	if o == nil || c.G.E.Power(id) >= c.G.E.Toughness(id) {
		return false
	}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, x := range g.Zone(state.ZBattlefield, p) {
			xo := g.Obj(x)
			if xo == nil || xo.Face() == nil {
				continue
			}
			switch xo.Face().Name {
			case "Bedrock Tortoise":
				if xo.Controller == o.Controller {
					return true
				}
			case "Bark of Doran":
				if xo.AttachedTo == id {
					return true
				}
			}
		}
	}
	return false
}

// ktoughPolicy attacks, blocks and crews whenever it can (so the games reach
// combat damage and the tap picks), else random; it counts the picks that
// exercise the toughness reads.
func ktoughPolicy(seed uint64, st *ktoughStats) Policy {
	rnd := RandomPolicy(seed)
	tapcost, _ := chooseCode("tapcost")
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActAttack || k == ActBlock || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G != nil {
			switch k := a.Kind(); {
			case k == ActAttack || k == ActBlock:
				if o, ok := c.G.GorgeOption(a); ok && ktoughUnderStatic(c, o.Obj) {
					st.toughInCombat++
				}
			case k == ActChoose+CanonKind(tapcost):
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Interface Ace" {
						st.aceCrews++
					}
				}
			}
		}
		return a
	})
}

// runTough plays the constructed matchup over seeds 1..seeds; off sets the
// DebugToughAssignOff mutation.
func runTough(t *testing.T, seeds int, off bool) ktoughStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tough")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugToughAssignOff
	dirac.DebugToughAssignOff = off
	defer func() { dirac.DebugToughAssignOff = old }()
	var st ktoughStats
	for mi, md := range ktoughDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ktough-a", "ktough-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, ktoughPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !off {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !off {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestToughLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runTough(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d power<toughness attack/block picks under a Tortoise/Bark static, %d Crew taps of Interface Ace", st.games, st.divs, st.stalls, st.toughInCombat, st.aceCrews)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.toughInCombat < 30 {
		t.Errorf("only %d toughness-in-combat picks, want >= 30", st.toughInCombat)
	}
	if st.aceCrews < 10 {
		t.Errorf("only %d Crew taps of Interface Ace, want >= 10", st.aceCrews)
	}
}

// TestToughMutation turns the toughness reads off: the kernel then assigns
// power and prices Crew by power, which gorge does not, so the games must
// diverge. A green mutation means the lockstep never reaches either read.
func TestToughMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runTough(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the reads off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
