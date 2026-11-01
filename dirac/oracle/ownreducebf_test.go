package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The battlefield own-ReduceCost$ lockstep: "this ability
// costs {N} less to activate ..." on a printed battlefield ability (gorge
// legal_walk_battlefield.go ownReduceCost: the generic part, floored at
// zero). Deck A reads Islands, Equipment, differently named lands and the
// active player; deck B reads a +1/+1-counter creature (Starport Security's
// Compare).
var kownreducebfDecks = [2][]cardset.Entry{
	{
		{Name: "Waterbending Scroll", Count: 4}, {Name: "Hylda's Crown of Winter", Count: 4},
		{Name: "The Lonely Mountain", Count: 4}, {Name: "Survey Mechan", Count: 4},
		{Name: "Swiftfoot Boots", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 6},
		{Name: "Grizzly Bears", Count: 4},
	},
	{
		{Name: "Starport Security", Count: 4}, {Name: "Skyknight Squire", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Qiqirn Merchant", Count: 4}, {Name: "Swamp", Count: 4},
	},
}

// kownreducebfPrinted is the printed generic part of each deck-A artifact's
// reduced ability.
var kownreducebfPrinted = map[string]int{
	"Waterbending Scroll": 6, "Hylda's Crown of Winter": 1, "The Lonely Mountain": 4, "Survey Mechan": 10,
}

// kownreducebfPolicy casts or activates whenever it can, else random, and
// counts the activations of a deck-A artifact whose gorge offer cost is below
// its printed generic part.
func kownreducebfPolicy(seed uint64, reduced map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
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
		if a.Kind() == ActActivate && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok && o.Cost != "" {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					if printed, ok := kownreducebfPrinted[name]; ok {
						gen := 0
						if f := strings.Fields(o.Cost); len(f) > 0 {
							gen, _ = strconv.Atoi(f[0])
						}
						if gen < printed {
							reduced[name]++
						}
					}
				}
			}
		}
		return a
	})
}

// runOwnreducebf plays seeds 1..seeds of the constructed matchup and
// returns the games, the divergences and the reduced activations.
func runOwnreducebf(t *testing.T, seeds int, report bool) (games, divs int, reduced map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ownreducebf")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kownreducebfDecks {
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
	tbl, _, _, err := set.Matchup(kownreducebfDecks[0], kownreducebfDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	reduced = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kownreducebf-a", "kownreducebf-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kownreducebfPolicy(seed, reduced), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
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
	return games, divs, reduced
}

func TestOwnreducebfLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, reduced := runOwnreducebf(t, 256, true)
	total := 0
	for _, n := range reduced {
		total += n
	}
	t.Logf("%d games, %d divergences, %d reduced activations %v", games, divs, total, reduced)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if total < 30 {
		t.Fatalf("only %d reduced activations (want >= 30)", total)
	}
}

// TestOwnreducebfMutation drops the reduction: gorge offers the reduced
// activation and the kernel does not, so the lockstep must diverge.
func TestOwnreducebfMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoOwnReduce = true
	defer func() { dirac.DebugNoOwnReduce = false }()
	games, divs, _ := runOwnreducebf(t, 64, false)
	if divs == 0 {
		t.Fatalf("no seed diverged without ReduceCost$ in %d games: the pool never activates a reduced battlefield ability", games)
	}
	t.Logf("mutation diverged (first divergence within %d games)", games)
}
