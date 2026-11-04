package oracle

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: a token copy of a
// clone-like card is asked the as-enters copy election and the answer is
// applied as the token enters. Feldon of the Third Path copies the Clone in
// the graveyard, Nightmare Shepherd copies a creature that died (a Clone
// among them), Cackling Counterpart copies a Clone that declined; a cast
// Clone, Stunt Double and Mockingbird copy what is on the battlefield.
//
// The deck is tuned: a shape outside this follow is kept out of the 192
// seeds, not modelled: a token copy of a copy of a clone card (Cackling
// Counterpart on a Clone that elected Mockingbird) stalls StallCopyAllAsk.
// (An elected clone that became a Nightmare Shepherd and dies together with
// another creature of its controller loses its look-back trigger in gorge;
// the kernel follows, TestClonetokShepherd.)
var kclonetokDeck = []cardset.Entry{
	{Name: "Clone", Count: 3}, {Name: "Stunt Double", Count: 1}, {Name: "Mockingbird", Count: 1},
	{Name: "Feldon of the Third Path", Count: 3}, {Name: "Nightmare Shepherd", Count: 2}, {Name: "Cackling Counterpart", Count: 3},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Silvercoat Lion", Count: 3}, {Name: "Ornithopter", Count: 2},
	{Name: "Doom Blade", Count: 2},
	{Name: "Island", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 2},
	{Name: "Memnite", Count: 4},
}

// kclonetokCounts is read off gorge's finished event log.
type kclonetokCounts struct {
	castCopy   atomic.Int64 // a cast clone that copied a permanent
	tokElect   atomic.Int64 // a token that became a copy through its own election
	tokDecline atomic.Int64 // a token copy of a clone card that entered as itself
}

func kclonetokFinish(c *kclonetokCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		// Names are read from the card of the event's object, never later: the
		// ids of tokens and of cards in a graveyard are not a card's own.
		elected := map[state.ObjID]bool{} // a permanent that became a copy
		asked := map[state.ObjID]bool{}   // a token whose copiable values carry the clone line
		tokens := map[state.ObjID]bool{}
		pending := false
		nAsked, nElect := 0, 0
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch {
			case e.Kind == events.CopyToken:
				o := g.E.G.Obj(e.Obj)
				pending = o != nil && !elected[e.Obj] && kclonetokCloneCards[cardName(o.Card)]
			case e.Kind == events.MoveZone && e.To == state.ZBattlefield && e.From == state.ZLibrary:
				tokens[e.Obj] = true // the only cards entering from a library here are minted tokens
				if pending {
					asked[e.Obj] = true
					nAsked++
					pending = false
				}
			case e.Kind == events.ClonePermanent && len(e.IDs) != 0:
				elected[e.Obj] = true
				if asked[e.Obj] {
					nElect++
				} else if !tokens[e.Obj] {
					c.castCopy.Add(1)
				}
			}
		}
		c.tokElect.Add(int64(nElect))
		c.tokDecline.Add(int64(nAsked - nElect))
	}
}

var kclonetokCloneCards = map[string]bool{"Clone": true, "Stunt Double": true, "Mockingbird": true}

func kclonetokRun(t *testing.T, seeds uint64, report bool, c *kclonetokCounts) (diverged bool, stalls int) {
	t.Helper()
	return kclonetokRunDeck(t, kclonetokDeck, seeds, report, c)
}

func kclonetokRunDeck(t *testing.T, deck []cardset.Entry, seeds uint64, report bool, c *kclonetokCounts) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("clonetok")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range deck {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], card)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck: %d cards", len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"clonetok-a", "clonetok-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := Policy(RandomPolicy(seed))
		if seed%2 == 1 {
			pol = HeuristicPolicy(seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25}
		if c != nil {
			rc.Finish = kclonetokFinish(c)
		}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return true, stalls
		}
	}
	return false, stalls
}

func TestClonetokLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var c kclonetokCounts
	div, stalls := kclonetokRun(t, 192, true, &c)
	t.Logf("192 seeds, %d divergences, %d stalls; cast clones that copied %d, token elections %d, token declines %d",
		b2i(div), stalls, c.castCopy.Load(), c.tokElect.Load(), c.tokDecline.Load())
	if div || stalls > 0 {
		return
	}
	for _, f := range []struct {
		s        string
		got, min int64
	}{
		{"cast clones that copied", c.castCopy.Load(), 20},
		{"token elections", c.tokElect.Load(), 10},
		{"token declines", c.tokDecline.Load(), 3},
	} {
		if f.got < f.min {
			t.Errorf("%s: %d, want at least %d", f.s, f.got, f.min)
		}
	}
}

// TestClonetokMutation: a token copy that asks nothing and enters as itself
// (the pre-5700d1e8e kernel) must make the lockstep diverge.
func TestClonetokMutation(t *testing.T) {
	dirac.DebugCloneTokenNoAsk = true
	defer func() { dirac.DebugCloneTokenNoAsk = false }()
	if diverged, _ := kclonetokRun(t, 64, false, nil); !diverged {
		t.Fatal("DebugCloneTokenNoAsk: no divergence in 64 seeds")
	}
}

// kclonetokShepherdDeck is built for one shape: a cast Clone or Stunt Double
// that elected Nightmare Shepherd dies in the same state-based action as
// another nontoken creature of its controller. CR 603.10a fires its "whenever
// another nontoken creature you control dies" trigger. No Cackling Counterpart:
// its token copy of a copy stalls StallCopyAllAsk.
var kclonetokShepherdDeck = []cardset.Entry{
	{Name: "Clone", Count: 3}, {Name: "Stunt Double", Count: 3}, {Name: "Mockingbird", Count: 1},
	{Name: "Feldon of the Third Path", Count: 3}, {Name: "Nightmare Shepherd", Count: 5},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Silvercoat Lion", Count: 3}, {Name: "Ornithopter", Count: 2},
	{Name: "Doom Blade", Count: 4}, {Name: "Memnite", Count: 1},
	{Name: "Island", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 2},
}

func TestClonetokShepherd(t *testing.T) {
	if isolate(t) {
		return
	}
	if div, stalls := kclonetokRunDeck(t, kclonetokShepherdDeck, 192, true, nil); div || stalls != 0 {
		t.Errorf("divergence %v, %d stalls", div, stalls)
	}
}
