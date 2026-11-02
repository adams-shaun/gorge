package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The any-event "can't become untapped" aura: Frozen in Ice
// ("Enchanted creature ... can't become untapped", R:Event$ Untap with no
// ValidStepTurnToController$) beside Twiddle, whose "untap" election is the
// effect-site Untap event gorge replaces (the untap step is the other).

var krepl3AnyUntapDecks = [2][]cardset.Entry{
	{
		{Name: "Frozen in Ice", Count: 4}, {Name: "Twiddle", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4}, {Name: "Island", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Walking Corpse", Count: 5}, {Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3}, {Name: "Swamp", Count: 12}, {Name: "Forest", Count: 8},
	},
}

type krepl3AnyUntapStats struct {
	frozenCast   int // Frozen in Ice picked at priority
	twiddleCast  int // Twiddle picked at priority with a frozen creature in play
	frozenAsks   int // tap/untap elections on a frozen creature, answered "untap"
	frozenUntaps int // events.Untap for an object enchanted by Frozen in Ice (must be 0)
}

// krepl3FrozenCount is how many Frozen in Ice on the battlefield enchant id.
func krepl3FrozenCount(g *state.Game, id state.ObjID) (n int) {
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, b := range g.Zone(state.ZBattlefield, pl) {
			if o := g.Obj(b); o != nil && o.AttachedTo == id && chName(g, b) == "Frozen in Ice" {
				n++
			}
		}
	}
	return n
}

func krepl3FrozenHost(g *state.Game, id state.ObjID) bool { return krepl3FrozenCount(g, id) > 0 }

// krepl3SingleHost reports whether a creature is enchanted by exactly one
// Frozen in Ice.
func krepl3SingleHost(g *state.Game) bool {
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, b := range g.Zone(state.ZBattlefield, pl) {
			if o := g.Obj(b); o != nil && o.AttachedTo != 0 && chName(g, b) == "Frozen in Ice" && krepl3FrozenCount(g, o.AttachedTo) == 1 {
				return true
			}
		}
	}
	return false
}

// krepl3AnyUntapPolicy casts Frozen in Ice whenever it can, holds Twiddle
// until a creature is frozen, aims Twiddle at the frozen creature and
// elects "untap" for it; everything else is krepl3Policy's.
func krepl3AnyUntapPolicy(seed uint64, st *krepl3AnyUntapStats) Policy {
	inner := krepl3Policy(seed, &krepl3Stats{})
	prng := rand.New(rand.NewPCG(seed, 0x616e79756e74)) // "anyunt"
	evAt := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		g := c.G.E.G
		evs := c.G.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for ; evAt < len(evs); evAt++ {
			if e := &evs[evAt]; e.Kind == events.Untap && krepl3FrozenHost(g, e.Obj) {
				st.frozenUntaps++
			}
		}
		switch c.D.Kind {
		case DPriority:
			hosts := krepl3SingleHost(g)
			var keep []Canon
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					if opt, ok := c.G.GorgeOption(a); ok {
						switch chName(g, opt.Obj) {
						case "Frozen in Ice":
							if prng.IntN(8) != 0 {
								st.frozenCast++
								return a
							}
						case "Twiddle":
							if !hosts {
								continue
							}
							if prng.IntN(8) != 0 {
								st.twiddleCast++
								return a
							}
						}
					}
				}
				keep = append(keep, a)
			}
			if len(keep) > 0 && len(keep) < len(c.D.Options) {
				d := *c.D
				d.Options = keep
				return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		default:
			if pd := c.G.Pending(); pd != nil && chName(g, pd.Source) == "Twiddle" {
				for _, a := range c.D.Options {
					opt, ok := c.G.GorgeOption(a)
					if !ok || !krepl3FrozenHost(g, opt.Obj) {
						continue
					}
					if krepl3FrozenCount(g, opt.Obj) > 1 {
						if opt.Kind == "tap" {
							return a // never an untap of a twice-frozen creature
						}
						continue
					}
					if opt.Kind == "untap" {
						st.frozenAsks++
						return a
					}
					if opt.Kind != "tap" && opt.Kind != "done" {
						return a // aim Twiddle at the frozen creature
					}
				}
			}
		}
		return inner.Pick(c)
	})
}

// runAnyuntapblockedAnyUntap plays the matchup in both seatings, seeds 1..seeds each;
// mutate expects divergences and does not report them.
func runAnyuntapblockedAnyUntap(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st krepl3AnyUntapStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("anykindcounters")
	if err != nil {
		t.Fatal(err)
	}
	for swap := 0; swap < 2; swap++ {
		md := krepl3AnyUntapDecks
		if swap == 1 {
			md = [2][]cardset.Entry{md[1], md[0]}
		}
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
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"krepl3-a", "krepl3-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, krepl3AnyUntapPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestAnyUntapBlockedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runAnyuntapblockedAnyUntap(t, 32, false)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.frozenAsks < 3 {
		t.Errorf("frozenAsks %d (want >= 3)", st.frozenAsks)
	}
	if st.frozenUntaps != 0 {
		t.Errorf("frozenUntaps %d (want 0): gorge untapped a creature enchanted by Frozen in Ice", st.frozenUntaps)
	}
}

// TestAnyUntapBlockedMutation turns the kernel's any-event block off: Twiddle's
// untap then frees the frozen creature the kernel's gorge twin keeps tapped,
// so the lockstep must diverge.
func TestAnyUntapBlockedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugAnyUntapOff
	dirac.DebugAnyUntapOff = true
	defer func() { dirac.DebugAnyUntapOff = old }()
	games, divs, _, _ := runAnyuntapblockedAnyUntap(t, 8, true)
	if divs == 0 {
		t.Errorf("mutation disabled the any-event untap block with no divergence in %d games", games)
	}
	t.Logf("mutation diverged %d of %d games", divs, games)
}
