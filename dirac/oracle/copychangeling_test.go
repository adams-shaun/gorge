package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copy exception "except it has changeling" lockstep (CR 707.9b,
// 702.73a): Dominion Infiltration makes token copies of X target creatures an
// opponent controls, each every creature type, so Goblin Chieftain's "other
// Goblin creatures you control get +1/+1 and have haste" reaches them. The
// spell's X target creatures are exactly X (CR 601.2c).

var kcopychangelingDecks = [2][]cardset.Entry{
	{
		{Name: "Dominion Infiltration", Count: 5}, {Name: "Goblin Chieftain", Count: 3},
		{Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 24},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 5}, {Name: "Gray Ogre", Count: 5},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	},
}

// kcopychangelingCounts counts what the games reach.
type kcopychangelingCounts struct {
	tokens int // token copies with every creature type
	lorded int // of those, ones that shared a battlefield with Goblin Chieftain
}

// kcopychangelingPolicy is the random policy that watches seat 0's
// battlefield at each decision for changeling token copies.
func kcopychangelingPolicy(seed uint64, st *kcopychangelingCounts) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	lorded := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			e := c.G.E
			ids := e.G.Zone(state.ZBattlefield, 0)
			chief := false
			for _, id := range ids {
				if ob := e.G.Obj(id); ob != nil && !ob.IsToken && ob.Face() != nil && ob.Face().Name == "Goblin Chieftain" {
					chief = true
				}
			}
			for _, id := range ids {
				ob := e.G.Obj(id)
				if ob == nil || !ob.IsToken || !e.Derived(id).AllCreatureTypes {
					continue
				}
				if !seen[id] {
					seen[id] = true
					st.tokens++
				}
				if chief && !lorded[id] {
					lorded[id] = true
					st.lorded++
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runCopychangeling(t *testing.T, seeds int, mutate func(), reset func()) (games, divs, stalls, firstDiv int, pc kcopychangelingCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copychangeling")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
		defer reset()
	}
	var decks [2][]*cards.Card
	for s, d := range kcopychangelingDecks {
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
	tbl, _, _, err := set.Matchup(kcopychangelingDecks[0], kcopychangelingDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcopychangeling-a", "kcopychangeling-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcopychangelingPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestCopychangelingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, _, pc := runCopychangeling(t, 256, nil, nil)
	t.Logf("%d games, %d divergences, %d stalls, changeling token copies %d, beside Goblin Chieftain %d",
		games, divs, stalls, pc.tokens, pc.lorded)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if pc.tokens < 20 {
		t.Errorf("only %d changeling token copies (need 20)", pc.tokens)
	}
	if pc.lorded < 5 {
		t.Errorf("only %d changeling token copies beside Goblin Chieftain (need 5)", pc.lorded)
	}
}

// TestCopychangelingMutation drops the changeling exception
// (dirac.DebugCopyChangelingOff): the tokens then miss the lord's +1/+1 and
// haste, and the games must diverge within 64 seeds.
func TestCopychangelingMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, first, _ := runCopychangeling(t, 64,
		func() { dirac.DebugCopyChangelingOff = true },
		func() { dirac.DebugCopyChangelingOff = false })
	if divs == 0 {
		t.Fatalf("mutation dropped the changeling exception in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games (first seed %d)", divs, games, first)
}
