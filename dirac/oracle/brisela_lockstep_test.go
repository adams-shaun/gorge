package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Brisela, Voice of Nightmares' "your opponents can't cast spells with mana
// value 3 or less" (S:Mode$ CantBeCast | ValidCard$ Card.cmcLE3) in lockstep.
//
// Gisela, the Broken Blade melds with Bruna, the Fading Light into Brisela at
// the beginning of its controller's end step (CR 701.42, 712.4). Hydroid
// Krasis is an {X}{G}{U} spell: its mana value is 2 at X = 0 (under the
// restriction) and 4 at X = 2 (above it). At the cast offer the kernel's X
// look-ahead must admit the cast because a payable X escapes (CR 601.3a), and
// the CR 601.2e recheck must reverse an announcement that leaves the mana
// value at or under 3 (CR 202.3e). Pods are 4 seats; the 2-seat run is the
// same decks at two.

var kbriselaDecks = [][]cardset.Entry{
	{
		{Name: "Gisela, the Broken Blade", Count: 4}, {Name: "Bruna, the Fading Light", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Hydroid Krasis", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Mind Stone", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
	},
}

// briselaName and xspellName are the witnesses: a Brisela on the battlefield
// and Hydroid Krasis on the stack.
const (
	briselaName = "Brisela, Voice of Nightmares"
	xspellName  = "Hydroid Krasis"
)

// briselaOn reports whether a Brisela permanent is on the battlefield and who
// controls it.
func briselaOn(g *GorgeSide) (state.PlayerID, bool) {
	if g == nil || g.E == nil || g.E.G == nil {
		return 0, false
	}
	for p := state.PlayerID(0); int(p) < len(g.E.G.Players); p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == briselaName {
				return p, true
			}
		}
	}
	return 0, false
}

// briselaXSeen counts, once per spell object, a Hydroid Krasis on the stack
// while an opponent controls Brisela: the offer's escape path actually ran.
type briselaXSeen struct {
	seen map[state.ObjID]bool
	n    int
}

// check is a RunConfig.Check: it watches every decision both engines agree on.
func (c *briselaXSeen) check(ref *GorgeSide, _ Side) *Divergence {
	if c.seen == nil {
		c.seen = map[state.ObjID]bool{}
	}
	if ref == nil || ref.E == nil || ref.E.G == nil {
		return nil
	}
	ctrl, ok := briselaOn(ref)
	if !ok {
		return nil
	}
	for _, id := range ref.E.G.Stack {
		o := ref.E.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != xspellName || o.Owner == ctrl || c.seen[id] {
			continue
		}
		c.seen[id] = true
		c.n++
	}
	return nil
}

// briselaReversals counts Hydroid Krasis casts gorge reversed: the CR 601.2e
// recheck's witness (the announced X left the spell's mana value at or under
// the bound).
func briselaReversals(g *GorgeSide) int {
	if g == nil || g.E == nil || g.E.G == nil {
		return 0
	}
	last := map[state.PlayerID]state.ObjID{}
	n := 0
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			last[ev.Player] = ev.Obj
		case events.SpellCastReversed:
			if o := g.E.G.Obj(last[ev.Player]); o != nil && o.Face() != nil && o.Face().Name == xspellName {
				n++
			}
		}
	}
	return n
}

// briselaPolicy prefers the X spell at priority and answers its X ask with
// the largest option on even seeds (a legal announcement that must be cast)
// and the smallest on odd seeds (an announcement the recheck must reverse).
func briselaPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && len(c.D.Options) > 0 {
			if d := c.G.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x" {
				if seed%2 == 0 {
					return c.D.Options[len(c.D.Options)-1]
				}
				return c.D.Options[0]
			}
		}
		if c.D.Kind == DPriority && c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == xspellName {
						return a
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runBriselaPods plays seats seats over seeds seeds, gorge leading, and returns
// the games, divergences and stalls plus the branch counts.
func runBriselaPods(t *testing.T, seats, seeds int, mutate bool) (games, divs, stalls, xUnder, reversed int) {
	t.Helper()
	c, _ := testCorpus(t)
	set, err := cardset.RawByName("brisela")
	if err != nil {
		t.Fatal(err)
	}
	md := make([][]cardset.Entry, seats)
	names := make([]string, seats)
	gd := make([][]*cards.Card, seats)
	for s := 0; s < seats; s++ {
		md[s], names[s] = kbriselaDecks[s%len(kbriselaDecks)], fmt.Sprint("kbrisela-", s)
		for _, e := range md[s] {
			cc, ok := c.Reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], cc)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	seen := &briselaXSeen{}
	if mutate {
		dirac.DebugCantCastNoXLookahead = true
		defer func() { dirac.DebugCantCastNoXLookahead = false }()
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchupN(c.Reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gameReversed int
		res := LockstepLed(m, seed, g, k, briselaPolicy(seed), RunConfig{
			Config: cfg, MaxDecisions: 6000, MaxTurns: 60, Check: seen.check,
			Finish: func(gs *GorgeSide) { gameReversed += briselaReversals(gs) },
		})
		games++
		reversed += gameReversed
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Logf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			if mutate {
				t.Logf("seed %d: mutation diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
				break // one divergence is the mutation's proof
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		CloseSide(k)
	}
	return games, divs, stalls, seen.n, reversed
}

func briselaLockstep(t *testing.T, seats, seeds int) {
	var st dirac.MeldStats
	dirac.DebugMeldStats = &st
	defer func() { dirac.DebugMeldStats = nil }()
	games, divs, stalls, xUnder, reversed := runBriselaPods(t, seats, seeds, false)
	t.Logf("%d seats: %d games, %d melds, %d splits; Hydroid Krasis under Brisela %d, reversed %d; stalls %d",
		seats, games, st.Melds, st.Splits, xUnder, reversed, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d/%d games diverged, %d stalls", divs, games, stalls)
	}
	if st.Melds == 0 {
		t.Fatalf("the pods never melded into Brisela")
	}
	if xUnder == 0 {
		t.Fatalf("no Hydroid Krasis was offered while an opponent controlled Brisela")
	}
	if reversed == 0 {
		t.Fatalf("no Hydroid Krasis cast was reversed by the CR 601.2e recheck")
	}
}

// TestBriselaCantBeCastLockstep2 and ...4: every game agrees decision by
// decision with gorge, a Brisela meld happens, an X spell is offered under
// the restriction, and a small-X announcement is reversed.
func TestBriselaCantBeCastLockstep2(t *testing.T) { briselaLockstep(t, 2, 40) }
func TestBriselaCantBeCastLockstep4(t *testing.T) { briselaLockstep(t, 4, 24) }

// TestBriselaCantBeCastMutation removes the offer's X look-ahead
// (DebugCantCastNoXLookahead): a cast gorge offers at an escaping X is
// withheld, so the lockstep must diverge.
func TestBriselaCantBeCastMutation(t *testing.T) {
	games, divs, _, _, _ := runBriselaPods(t, 4, 24, true)
	if divs == 0 {
		t.Fatalf("DebugCantCastNoXLookahead in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
