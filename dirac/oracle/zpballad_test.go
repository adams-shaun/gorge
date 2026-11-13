package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The value-bound toughness filter lockstep (CR 701.8,
// 208.1, 613): Zero Point Ballad destroys every creature whose CURRENT
// toughness is X or less (PredToughnessLEV), X being the paid X. Holy
// Strength (+1/+2) and Dead Weight (-2/-2) put pumped and shrunk creatures
// of mixed printed toughness on the battlefield, so a kernel that tests the
// printed toughness, the wrong side of X, or a stale X diverges.
var kzpbDecks = [2][]cardset.Entry{
	{ // A: Ballads, auras and creatures of toughness 1, 2 and 3
		{Name: "Zero Point Ballad", Count: 6}, {Name: "Holy Strength", Count: 4}, {Name: "Dead Weight", Count: 4},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Swamp", Count: 9}, {Name: "Plains", Count: 8},
	},
	{ // B: creatures of toughness 1, 2 and 3 and burn
		{Name: "Zero Point Ballad", Count: 2}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 3},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 5},
	},
}

type kzpbCast struct {
	x            int
	mixed, aurad bool
}

type kzpbTally struct {
	live   map[state.ObjID]*kzpbCast // Ballads on the stack, X as last seen (it is set after the cast begins)
	byX    [8]int                    // resolved Ballad casts by paid X (7: seven or more)
	mixed  int                       // casts with creatures of two or more printed toughnesses in play
	auraed int                       // casts with an Aura on the battlefield
}

func (c *kzpbTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	onStack := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		if sourceCardName(g, id) != "Zero Point Ballad" {
			continue
		}
		o := g.Obj(id)
		if o == nil || o.Ability != nil {
			continue
		}
		onStack[id] = true
		cs := c.live[id]
		if cs == nil {
			cs = &kzpbCast{}
			c.live[id] = cs
		}
		cs.x = int(o.X)
		tough := map[int]bool{}
		cs.aurad = false
		for pl := state.PlayerID(0); pl < 2; pl++ {
			for _, b := range g.Zone(state.ZBattlefield, pl) {
				bo := g.Obj(b)
				if bo == nil || bo.Face() == nil {
					continue
				}
				for _, ty := range bo.Face().Types {
					switch ty {
					case "Creature":
						tough[bo.Face().Toughness()] = true
					case "Aura":
						cs.aurad = true
					}
				}
			}
		}
		cs.mixed = len(tough) >= 2
	}
	for id, cs := range c.live {
		if onStack[id] {
			continue
		}
		delete(c.live, id)
		c.byX[min(cs.x, 7)]++
		if cs.mixed {
			c.mixed++
		}
		if cs.aurad {
			c.auraed++
		}
	}
	return nil
}

func TestZpBalladLockstep(t *testing.T) {
	t.Parallel()
	cardsettest.Require(t, "gybf")
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gybf")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kzpbDecks {
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
	tbl, _, _, err := set.Matchup(kzpbDecks[0], kzpbDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tl := &kzpbTally{live: map[state.ObjID]*kzpbCast{}}
	divs, stalls := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kzpb-a", "kzpb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		clear(tl.live)
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tl.check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d divergences, %d stalls; Ballad casts by X %v; %d with mixed toughness, %d with an Aura in play", divs, stalls, tl.byX, tl.mixed, tl.auraed)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, x := range []int{0, 1, 3} {
		if tl.byX[x] < 3 {
			t.Errorf("X=%d: %d Ballad casts, want >= 3", x, tl.byX[x])
		}
	}
	if tl.mixed < 5 || tl.auraed < 3 {
		t.Errorf("%d mixed-toughness casts, %d with an Aura; want >= 5 and >= 3", tl.mixed, tl.auraed)
	}
}
