package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The "X target" lockstep (CR 601.2b, 601.2c): Icy Blast, Meteor Blast,
// Distorting Wake and Death Denied target exactly the announced X, so X is
// offered only up to the legal targets there are (gorge
// targetsAnnounceableAt), and the target ask takes exactly X.

var ktgtxcountDecks = [2][]cardset.Entry{
	{
		{Name: "Icy Blast", Count: 3}, {Name: "Meteor Blast", Count: 3}, {Name: "Distorting Wake", Count: 2},
		{Name: "Death Denied", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Scathe Zombies", Count: 3},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 5}, {Name: "Gray Ogre", Count: 4},
		{Name: "Forest", Count: 13}, {Name: "Mountain", Count: 12},
	},
}

// ktgtxcountCounts counts what the games reach.
type ktgtxcountCounts struct {
	multi int // X spells put on the stack with two or more targets
}

// ktgtxcountSpells are the pool's "X target" spells.
var ktgtxcountSpells = map[string]bool{"Icy Blast": true, "Meteor Blast": true, "Distorting Wake": true, "Death Denied": true}

// ktgtxcountPolicy is the random policy that watches the stack at each
// decision for an "X target" spell with two or more targets.
func ktgtxcountPolicy(seed uint64, st *ktgtxcountCounts) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for _, id := range g.Stack {
				ob := g.Obj(id)
				if ob == nil || ob.Face() == nil || !ktgtxcountSpells[ob.Face().Name] || seen[id] {
					continue
				}
				if len(ob.Targets) >= 2 {
					seen[id] = true
					st.multi++
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runTgtxcount(t *testing.T, seeds int, mutate func(), reset func()) (games, divs, stalls, firstDiv int, pc ktgtxcountCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tgtxcount")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
		defer reset()
	}
	var decks [2][]*cards.Card
	for s, d := range ktgtxcountDecks {
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
	tbl, _, _, err := set.Matchup(ktgtxcountDecks[0], ktgtxcountDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktgtxcount-a", "ktgtxcount-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, ktgtxcountPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if firstDiv == 0 {
				firstDiv = int(seed)
			}
			if mutate == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, firstDiv, pc
}

func TestTgtxcountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, _, pc := runTgtxcount(t, 256, nil, nil)
	t.Logf("%d games, %d divergences, %d stalls, X spells with two or more targets %d", games, divs, stalls, pc.multi)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if pc.multi < 20 {
		t.Errorf("only %d X spells with two or more targets (need 20)", pc.multi)
	}
}

// TestTgtxcountMutation evaluates each "X target" slot's maximum one too high
// (dirac.DebugTgtBoundsMaxPlus1): the kernel then asks for a target past X,
// and the games must diverge within 64 seeds.
func TestTgtxcountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, first, _ := runTgtxcount(t, 64,
		func() { dirac.DebugTgtBoundsMaxPlus1 = true },
		func() { dirac.DebugTgtBoundsMaxPlus1 = false })
	if divs == 0 {
		t.Fatalf("mutation widened the X target slots in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games (first seed %d)", divs, games, first)
}
