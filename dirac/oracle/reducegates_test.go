package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ReduceCost turn and first-spell gates' lockstep:
// Mental Modulation's PlayerTurn$ You, Geyser Drake's Condition$
// NotPlayerTurn on a non-self static and Radagast of Rhosgobel's and Serah
// Farron's OnlyFirstSpell$ True. The kernel's conditions over the cast ring
// must price every cast the way gorge's costTurnGateHolds, costConditionHolds
// and onlyFirstSpellUsed do.

var kreducegatesDecks = [][2][]cardset.Entry{{
	// A: G/W first-spell reducers and legendary creatures
	{
		{Name: "Radagast of Rhosgobel", Count: 4}, {Name: "Serah Farron", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Ambush Viper", Count: 4}, {Name: "Bounding Wolf", Count: 4}, {Name: "Benalish Knight", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 6},
	},
	// B: U/G Geyser Drake, Mental Modulation and flash creatures
	{
		{Name: "Geyser Drake", Count: 4}, {Name: "Mental Modulation", Count: 4}, {Name: "Ambush Viper", Count: 4},
		{Name: "Bounding Wolf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Radagast of Rhosgobel", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	},
}, {
	// B against A with the seats swapped
	{
		{Name: "Geyser Drake", Count: 4}, {Name: "Mental Modulation", Count: 4}, {Name: "Ambush Viper", Count: 4},
		{Name: "Bounding Wolf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Radagast of Rhosgobel", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Radagast of Rhosgobel", Count: 4}, {Name: "Serah Farron", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Ambush Viper", Count: 4}, {Name: "Bounding Wolf", Count: 4}, {Name: "Benalish Knight", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 6},
	},
}}

// kreducegatesStats is what the policy counts across the games.
type kreducegatesStats struct {
	drakeOppCasts  int // casts posed on the opponent's turn with a Geyser Drake on the caster's battlefield
	firstCreature  int // Radagast first-creature casts posed (a Radagast out, no earlier spell this turn)
	secondCreature int // a second creature cast posed in the same turn with a Radagast out
	modOwnTurn     int // Mental Modulation casts posed on its controller's turn
}

// kreducegatesPolicy prefers casting (so several spells land per turn),
// else random, and counts the carrier states the gorge side poses.
func kreducegatesPolicy(seed uint64, st *kreducegatesStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	lastTurn := int32(-1)
	var cast [2]int // spells each player has put on the stack this turn
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
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
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		if t := int32(g.Turn); t != lastTurn {
			lastTurn = t
			cast = [2]int{}
		}
		if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast {
			p := g.Priority
			if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
				f := ob.Face()
				radagast := false
				drake := false
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if bo := g.Obj(id); bo != nil && bo.Face() != nil {
						switch bo.Face().Name {
						case "Radagast of Rhosgobel":
							radagast = true
						case "Geyser Drake":
							drake = true
						}
					}
				}
				isCreature := f.IsCreature()
				if drake && g.Active != p {
					st.drakeOppCasts++
				}
				if radagast && isCreature {
					if cast[p] == 0 {
						st.firstCreature++
					} else {
						st.secondCreature++
					}
				}
				if f.Name == "Mental Modulation" && g.Active == p {
					st.modOwnTurn++
				}
			}
			cast[p]++
		}
		return a
	})
}

// runReducegates plays the constructed matchups and returns the games, the
// divergences, the stalls and the carrier counts. mutate stops the kernel
// hiding the in-flight spell's push (the mutation must diverge).
func runReducegates(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kreducegatesStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("reducegates")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugReduceGatesNoFirst
	dirac.DebugReduceGatesNoFirst = mutate
	defer func() { dirac.DebugReduceGatesNoFirst = old }()
	for mi, md := range kreducegatesDecks {
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
			m, err := NewMatchup(reg, [2]string{"kreducegates-a", "kreducegates-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kreducegatesPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestReducegatesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runReducegates(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, Geyser Drake opponent-turn casts %d, Radagast first creatures %d, second creatures %d, Mental Modulation own-turn casts %d",
		games, divs, stalls, st.drakeOppCasts, st.firstCreature, st.secondCreature, st.modOwnTurn)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.drakeOppCasts < 10 {
		t.Errorf("only %d Geyser Drake opponent-turn casts (want >= 10)", st.drakeOppCasts)
	}
	if st.firstCreature < 10 {
		t.Errorf("only %d Radagast first-creature casts (want >= 10)", st.firstCreature)
	}
	if st.secondCreature < 5 {
		t.Errorf("only %d Radagast second-creature casts (want >= 5)", st.secondCreature)
	}
	if st.modOwnTurn < 5 {
		t.Errorf("only %d Mental Modulation own-turn casts (want >= 5)", st.modOwnTurn)
	}
}

// TestReducegatesMutation stops the kernel hiding the in-flight spell's own
// push while its cost is composed: the first covered spell then sees itself
// in the ledger and pays full price, where gorge's onlyFirstSpellUsed skips
// the push being priced. The games must diverge.
func TestReducegatesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runReducegates(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation ran %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

// The Melek deck (both sides, 40 cards): 4 Melek put the reducer on the
// battlefield, 8 Inspiration ({3}{U}) are the covered spells and the
// first-spell ledger carriers, 8 Grizzly Bears ({1}{G}) are the
// non-covered creature casts that must not spend the discount, and the
// basics pay {3}{U}, {3}{U}{R} and {1}{G}.
var kmelekDeck = []cardset.Entry{
	{Name: "Melek, Reforged Researcher", Count: 4},
	{Name: "Inspiration", Count: 8},
	{Name: "Grizzly Bears", Count: 8},
	{Name: "Island", Count: 12},
	{Name: "Mountain", Count: 4},
	{Name: "Forest", Count: 4},
}
var kmelekDecks = [2][]cardset.Entry{kmelekDeck, kmelekDeck}

// TestReducegatesValidSpellLockstep plays the Melek matchup over the
// generic cast-first policy: gorge prices Melek's first-instant {3}
// discount and the kernel must agree on every field.
func TestReducegatesValidSpellLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "reducegates", kmelekDecks, 300, "KMELEK_SEEDS")
}

// TestReducegatesValidSpellMutation stops the kernel hiding the
// in-flight spell's own push while its cost is composed: the first covered
// instant sees itself and pays full price, so a Melek cast must diverge
// against gorge (the proof the lockstep deck reaches the carrier).
func TestReducegatesValidSpellMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugReduceGatesNoFirst
	dirac.DebugReduceGatesNoFirst = true
	defer func() { dirac.DebugReduceGatesNoFirst = old }()
	if lockstepDiverges(t, "reducegates", kmelekDecks, 100) {
		return
	}
	t.Fatal("no seed diverged with the in-flight cast counted as its own prior cast: " +
		"the Melek deck did not reach a first instant/sorcery")
}

// The recast deck (both sides, 40 cards): Radagast's first creature spell each
// turn costs {2} less. Unsummon returns a creature spent on that discount, so
// its recast is a second creature cast of the same object (an earlier cast is
// a real spell); Rimrock Knight's Boulder Rush is an instant, so casting it as the
// Adventure spends nothing and the Knight cast from exile is still the first
// creature spell.
var krecastDeck = []cardset.Entry{
	{Name: "Radagast of Rhosgobel", Count: 4},
	{Name: "Rimrock Knight", Count: 6},
	{Name: "Unsummon", Count: 6},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Forest", Count: 7},
	{Name: "Mountain", Count: 7},
	{Name: "Island", Count: 6},
}
var krecastDecks = [2][]cardset.Entry{krecastDeck, krecastDeck}

// TestReducegatesRecastLockstep plays the recast deck over the generic
// cast-first policy: gorge counts an earlier cast of the object being priced
// and reads an Adventure cast as its instant half; the kernel must agree on
// every field.
func TestReducegatesRecastLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "reducegates", krecastDecks, 300, "KRECAST_SEEDS")
}
