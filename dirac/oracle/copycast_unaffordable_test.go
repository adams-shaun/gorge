package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A paid Play CopyCard$ cast (Q, Stacking the Deck's exile grant) that the
// caster cannot afford still begins: gorge mints the copy, opens its manual
// mana window, taps what the autopay surface offers, then reverses the cast
// (CR 733.1), keeping the taps; the copy ceases (CR 704.5d). The kernel used
// to latch StallCopyCast before any source was tapped. This policy activates
// the grant even when the copy is unaffordable, and counts those activations
// so the test proves it reached the reversal.
func copycastUnaffordablePolicy(seed uint64, acts map[string]int, short *int) Policy {
	inner := poisonhighPolicy(seed, map[string]int{}, acts)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "ability" && o.SVar != "" {
					g := c.G.E.G
					if len(g.Stack) != 0 {
						continue // one copy at a time: the payable case is TestQStackingTheDeckLockstep's
					}
					if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZExile && ob.Face() != nil {
						mana := int(g.Players[o.Player].Pool.Total())
						for _, id := range g.Zone(state.ZBattlefield, o.Player) {
							if l := g.Obj(id); l != nil && l.Face() != nil && l.Face().IsLand() && !l.Tapped {
								mana++
							}
						}
						if mana < int(ob.Face().ManaValue()) {
							*short++
						}
					}
				}
			}
			buf = append(buf, a)
		}
		d := *c.D
		d.Options = buf
		return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestCopyCastUnaffordableLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	decksIn := [2][]cardset.Entry{
		{{Name: "Q, Stacking the Deck", Count: 4}, {Name: "Hill Giant", Count: 12}, {Name: "Mountain", Count: 24}},
		{{Name: "Q, Stacking the Deck", Count: 4}, {Name: "Hill Giant", Count: 12}, {Name: "Mountain", Count: 24}},
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("qdeck")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range decksIn {
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
	tbl, _, _, err := set.Matchup(decksIn[0], decksIn[1])
	if err != nil {
		t.Fatal(err)
	}
	acts := map[string]int{}
	short := 0
	const seeds = 128
	for seed := uint64(1); seed <= seeds; seed++ {
		// Pair 1235 runs the autopay surface; the manual window is the
		// shape the kernel must model there.
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"copycast-a", "copycast-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, copycastUnaffordablePolicy(seed, acts, &short), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v\nactions %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 8), res.Actions[lo:d.At])
		}
	}
	// The precondition: at least one activated copy was unaffordable, so the
	// games really took the window-then-reversal path.
	if short == 0 {
		t.Fatalf("no unaffordable exile-granted copy cast in %d games; activations by card %v", seeds, acts)
	}
	t.Logf("%d unaffordable copy casts, activations by card %v", short, acts)
}
