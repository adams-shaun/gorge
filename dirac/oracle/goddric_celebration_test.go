package oracle

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Goddric lockstep: Goddric, Cloaked Reveler's Celebration static is a
// Self override gated on two nonland permanents entering under its
// controller's control this turn. While the gate holds it makes Goddric a
// Dragon 4/4 with flying and drops its other creature types (RemoveCreature-
// Types$ without RemoveCardTypes$): Human Noble go, Legendary Creature stay,
// Dragon comes. Off the gate it is the printed Human Noble 3/3. The decks
// carry cheap nonland permanents (to open the gate), Human readers (Thalia's
// Lieutenant, Champion of the Parish) and Dragon readers (Dragonlord's
// Servant), on a pool of their own (fixtures/goddric).

var kgoddricMu = [][2][]cardset.Entry{{
	{
		{Name: "Goddric, Cloaked Reveler", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Mogg Fanatic", Count: 3}, {Name: "Dragonlord's Servant", Count: 3},
		{Name: "Thalia's Lieutenant", Count: 3}, {Name: "Champion of the Parish", Count: 3},
		{Name: "Elite Vanguard", Count: 3}, {Name: "Shivan Dragon", Count: 2},
		{Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Silvercoat Lion", Count: 4},
		{Name: "Goblin Chieftain", Count: 3}, {Name: "Raging Goblin", Count: 3},
		{Name: "Dragon Whelp", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 4},
	},
}, {
	{
		{Name: "Goddric, Cloaked Reveler", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Mogg Fanatic", Count: 4}, {Name: "Dragonlord's Servant", Count: 3},
		{Name: "Thalia's Lieutenant", Count: 3}, {Name: "Champion of the Parish", Count: 3},
		{Name: "Dragon Whelp", Count: 2}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Goddric, Cloaked Reveler", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Mogg Fanatic", Count: 4}, {Name: "Dragonlord's Servant", Count: 3},
		{Name: "Thalia's Lieutenant", Count: 3}, {Name: "Champion of the Parish", Count: 3},
		{Name: "Dragon Whelp", Count: 2}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Plains", Count: 3},
	},
}}

// kgoddricStats counts, per decision on gorge's board, the Goddrics whose
// Celebration gate holds (a Dragon, no Human or Noble, still a Legendary
// Creature) and fails (the printed Human Noble).
type kgoddricStats struct{ on, off int }

func kgoddricScan(g *GorgeSide, st *kgoddricStats, bad *[]string) {
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Goddric, Cloaked Reveler" {
			continue
		}
		ty := g.E.Derived(o.ID).Types
		has := func(w string) bool { return slices.Contains(ty, w) }
		switch {
		case has("Dragon"):
			st.on++
			if has("Human") || has("Noble") || !has("Creature") || !has("Legendary") {
				*bad = append(*bad, strings.Join(ty, " "))
			}
		default:
			st.off++
			if !has("Human") || !has("Noble") || !has("Creature") {
				*bad = append(*bad, strings.Join(ty, " "))
			}
		}
	}
}

// kgoddricPolicy casts and activates whenever it can (so permanents arrive
// and the gate opens), else plays random, and scans gorge's board.
func kgoddricPolicy(seed uint64, st *kgoddricStats, bad *[]string) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			kgoddricScan(c.G, st, bad)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runGoddric plays the constructed matchups; keep skips the creature-type
// strip (the mutation must diverge).
func runGoddric(t *testing.T, seeds int, keep bool) (games, divs, stalls int, st kgoddricStats, bad []string) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("goddric")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRmCreatSubKeep
	dirac.DebugRmCreatSubKeep = keep
	defer func() { dirac.DebugRmCreatSubKeep = old }()
	matchupChunks(t, len(kgoddricMu), 1, uint64(seeds), 64, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		mi, md := c.mi, kgoddricMu[c.mi]
		var g, dv, sl int
		var cst kgoddricStats
		var cbad []string
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
		for seed := c.lo; seed <= c.hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"goddric-a", "goddric-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			gs, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, gs, k, kgoddricPolicy(seed, &cst, &cbad), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			g++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				sl++
			}
			if d := res.Div; d != nil {
				dv++
				if !keep {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
		mu.Lock()
		defer mu.Unlock()
		games, divs, stalls = games+g, divs+dv, stalls+sl
		st.on += cst.on
		st.off += cst.off
		bad = append(bad, cbad...)
	})
	return games, divs, stalls, st, bad
}

func TestGoddricCelebrationLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st, bad := runGoddric(t, 192, false)
	t.Logf("%d games, %d divergences, %d stalls, Goddric seen %d decisions with the gate holding, %d failing",
		games, divs, stalls, st.on, st.off)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if len(bad) != 0 {
		t.Fatalf("gorge's Goddric types are not the Celebration shape: %q", bad[:min(len(bad), 5)])
	}
	if st.on < 20 || st.off < 20 {
		t.Errorf("gate holding %d, failing %d decisions: both arms need 20", st.on, st.off)
	}
}

// TestGoddricCelebrationMutation skips the creature-type strip (the
// add-only reading): a celebrating Goddric stays a Human Noble Dragon, so
// the games must diverge.
func TestGoddricCelebrationMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runGoddric(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation kept the creature types in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
