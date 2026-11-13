package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Token copies made one target at a time (CR 608.2c, 707.9): Dominion
// Infiltration copies X target creatures an opponent controls, and a copy of
// Diamond Mare or Nyota Uhura is asked its colour or number as it enters,
// before the targets after it get their copies, the spell still on the stack.

var kcopyeachaskDecks = [2][]cardset.Entry{
	{
		{Name: "Dominion Infiltration", Count: 8}, {Name: "Raging Goblin", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 22},
	},
	{
		{Name: "Diamond Mare", Count: 7}, {Name: "Nyota Uhura, Communications", Count: 3},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 11}, {Name: "Island", Count: 11},
	},
}

// kcopyeachaskCounts counts what the games reach.
type kcopyeachaskCounts struct {
	tokens int // token copies of a creature with an as-enters choice
	midAsk int // decisions taken while Dominion Infiltration resolves with a token already made
}

// kcopyeachaskPolicy is the random policy that watches seat 0's battlefield
// and the stack at each decision.
func kcopyeachaskPolicy(seed uint64, st *kcopyeachaskCounts) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			e := c.G.E
			made := 0
			for _, id := range e.G.Zone(state.ZBattlefield, 0) {
				ob := e.G.Obj(id)
				if ob == nil || !ob.IsToken || ob.Face() == nil {
					continue
				}
				made++
				if n := ob.Face().Name; !seen[id] && (n == "Diamond Mare" || n == "Nyota Uhura, Communications") {
					seen[id] = true
					st.tokens++
				}
			}
			for _, id := range e.G.Zone(state.ZStack, 0) {
				if ob := e.G.Obj(id); made > 0 && ob != nil && ob.Face() != nil && ob.Face().Name == "Dominion Infiltration" {
					st.midAsk++
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runCopyeachask(t *testing.T, seeds int, mutate func(), reset func()) (games, divs, stalls, firstDiv int, pc kcopyeachaskCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copyeachask")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
		defer reset()
	}
	var decks [2][]*cards.Card
	for s, d := range kcopyeachaskDecks {
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
	tbl, _, _, err := set.Matchup(kcopyeachaskDecks[0], kcopyeachaskDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcopyeachask-a", "kcopyeachask-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcopyeachaskPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestCopyeachaskLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, _, pc := runCopyeachask(t, 256, nil, nil)
	t.Logf("%d games, %d divergences, %d stalls, copies of an as-enters chooser %d, decisions mid-resolution %d",
		games, divs, stalls, pc.tokens, pc.midAsk)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if pc.tokens < 20 {
		t.Errorf("only %d token copies of an as-enters chooser (need 20)", pc.tokens)
	}
	if pc.midAsk < 5 {
		t.Errorf("only %d decisions with Dominion Infiltration resolving and a token made (need 5)", pc.midAsk)
	}
}

// TestCopyeachaskMutation runs the loop on past the ask
// (dirac.DebugCopyEachNoPark): every target's copy is made before the choice,
// and the games must diverge within 64 seeds.
func TestCopyeachaskMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, first, _ := runCopyeachask(t, 64,
		func() { dirac.DebugCopyEachNoPark = true },
		func() { dirac.DebugCopyEachNoPark = false })
	if divs == 0 {
		t.Fatalf("mutation ran the loop past the ask in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games (first seed %d)", divs, games, first)
}
