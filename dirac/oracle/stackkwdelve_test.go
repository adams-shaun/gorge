package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The granted Delve of the AffectedZone$ Stack keyword static: Teval,
// Arbiter of Virtue ("spells you cast have delve"). No card in the pool
// prints delve, so every delve ask is the grant's. The policy casts Teval
// first, then prefers the expensive creatures (the graveyard fillers feed
// the delve), and picks every offered exile.

var kstackkwDelveDeck = []cardset.Entry{
	{Name: "Teval, Arbiter of Virtue", Count: 4},
	{Name: "Shivan Dragon", Count: 2}, {Name: "Air Elemental", Count: 2}, {Name: "Mahamoti Djinn", Count: 2},
	{Name: "Giant Spider", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Gray Ogre", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Walking Corpse", Count: 1},
	{Name: "Opt", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Brainstorm", Count: 1}, {Name: "Impulse", Count: 2}, {Name: "Divination", Count: 1},
	{Name: "Iridescent Tiger", Count: 3}, {Name: "Mind Stone", Count: 1}, {Name: "Sol Ring", Count: 1},
	{Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 2}, {Name: "Plains", Count: 1},
}

var kstackkwDelveBig = map[string]bool{"Shivan Dragon": true, "Air Elemental": true, "Mahamoti Djinn": true, "Giant Spider": true, "Hill Giant": true}

// kstackkwDelveTeval reports whether a Teval is on the battlefield.
func kstackkwDelveTeval(g *GorgeSide) bool {
	for p := state.PlayerID(0); int(p) < len(g.E.G.Players); p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			if o := g.E.G.Obj(id); o != nil && o.Face().Name == "Teval, Arbiter of Virtue" {
				return true
			}
		}
	}
	return false
}

func kstackkwDelvePolicy(seed uint64, cnt map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x64656c7665)) // "delve"
	var buf []Canon
	run := 0 // delve picks made in the open ask
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Kind != DChoose {
			run = 0
		}
		if c.D.Kind == DChoose && run > 0 && prng.IntN(3) == 0 {
			// close the ask early: the generic the picks leave is paid
			// from lands through gorge's manual window
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					run = 0
					cnt["delve-short"]++
					return a
				}
			}
		}
		if cp, ok := chooseCode("exile"); ok && c.D.Kind == DChoose {
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActChoose+CanonKind(cp) && o.Kind == "exile" {
					if kstackkwDelveTeval(c.G) {
						cnt["delve-picks"]++
					}
					run++
					return a
				}
			}
		}
		if c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, name := range []string{"Teval, Arbiter of Virtue", "Iridescent Tiger"} {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && refCardName(c.G, a.Src()) == name {
						return a
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && kstackkwDelveBig[refCardName(c.G, a.Src())] && prng.IntN(4) != 0 {
					cnt["big-casts"]++
					return a
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		}
		return rnd.Pick(c)
	})
}

// runStackkwDelve plays seeds 1..seeds of the Teval deck against itself.
func runStackkwDelve(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stackkwdelve")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kstackkwDelveDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for _, kw := range c.Faces[0].Keywords {
			if strings.HasPrefix(kw, "Delve") {
				t.Fatalf("%s prints delve: every ask must be the grant's", e.Name)
			}
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck has %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(kstackkwDelveDeck, kstackkwDelveDeck)
	if err != nil {
		t.Fatal(err)
	}
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstackkw-delve-a", "kstackkw-delve-b"}, [2][]*cards.Card{deck, deck}, 1)
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
		res := LockstepLed(m, seed, g, k, kstackkwDelvePolicy(seed, cnt), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

func TestStackkwDelveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runStackkwDelve(t, 128, true)
	t.Logf("%d games, %d divergences, %d stalls, counts %v", games, divs, stalls, cnt)
	if cnt["delve-picks"] < 10 {
		t.Errorf("only %d accepted delve picks with Teval in play (want >= 10)", cnt["delve-picks"])
	}
	if cnt["delve-short"] < 3 {
		t.Errorf("only %d delve asks closed early (want >= 3): no leftover generic paid from lands", cnt["delve-short"])
	}
}

// TestStackkwDelveMutation makes the kernel ignore the keyword grant: the
// delve-credited casts and asks vanish, so the lockstep must diverge.
func TestStackkwDelveMutation(t *testing.T) {
	old := dirac.DebugStackKwOff
	dirac.DebugStackKwOff = true
	defer func() { dirac.DebugStackKwOff = old }()
	games, divs, _, _ := runStackkwDelve(t, 32, false)
	if divs == 0 {
		t.Fatalf("grant ignored in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
