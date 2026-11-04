package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The each-colour tap-trigger lockstep: a play only Bloom Tender's or
// Faeburrow Elder's each-colour mana pays (the payment planner prices it as a
// fixed production, dirac mana_eachamong.go) taps creatures Rewrite History
// hears. Each tap's trigger goes on the stack at the next priority, one at a
// time (CR 603.3), and a sorcery-speed play waits for them to resolve (CR
// 307.1). Two such taps in one prefix are the shape the granted-mana prefix
// once left pending together, and that a scripted prefix still leaves.

var kgaptapsDeck = []cardset.Entry{
	{Name: "Bloom Tender", Count: 4}, {Name: "Faeburrow Elder", Count: 4}, {Name: "Rewrite History", Count: 4},
	{Name: "Savannah Lions", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Serra Angel", Count: 3},
	{Name: "Craw Wurm", Count: 2}, {Name: "Air Elemental", Count: 2},
	{Name: "Plains", Count: 5}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 4},
}

// kgaptapsPolicy casts whenever it can, else plays randomly.
func kgaptapsPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
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

// gapTapPrefixes counts, in gorge's log, the actions whose prefix tapped two or
// more of Bloom Tender and Faeburrow Elder for mana and stacked a Rewrite
// History trigger after each tap.
func gapTapPrefixes(g *GorgeSide) int {
	evs := g.E.L.Events
	n, taps, trigs := 0, 0, 0
	name := func(i int) string {
		if o := g.E.G.Obj(evs[i].Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for i := range evs {
		switch evs[i].Kind {
		case events.Tap:
			if nm := name(i); nm == "Bloom Tender" || nm == "Faeburrow Elder" {
				taps++
			}
		case events.TriggerPush:
			if name(i) == "Rewrite History" && taps > trigs {
				trigs++
			}
		case events.PutOnStack:
			if taps >= 2 && trigs >= 2 {
				n++
			}
			taps, trigs = 0, 0
		}
	}
	return n
}

func runGaptaps(t *testing.T, kgaptapsDeck []cardset.Entry, seeds int, quiet bool) (games, divs, stalls, reached int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gaptaps")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kgaptapsDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(kgaptapsDeck, kgaptapsDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kgaptaps-a", "kgaptaps-b"}, [2][]*cards.Card{deck, deck}, 1)
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
		res := LockstepLed(m, seed, g, k, kgaptapsPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		reached += gapTapPrefixes(g)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if quiet {
				continue
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return games, divs, stalls, reached
}

func TestGaptapsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, reached := runGaptaps(t, kgaptapsDeck, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, %d prefixes with two gap taps each heard by Rewrite History", games, divs, stalls, reached)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if reached < 5 {
		t.Errorf("only %d prefixes with two heard gap taps (need 5)", reached)
	}
}

// TestGaptapsLockstepMutation: with the each-colour planner view dropped
// (view 0, no unit) the kernel scripts the sources gorge's planner prices.
func TestGaptapsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.SetDebugEachAmongMutate(1)
	defer dirac.SetDebugEachAmongMutate(0)
	if _, divs, _, _ := runGaptaps(t, kgaptapsDeck, 32, true); divs == 0 {
		t.Error("mutation 1: no seed diverged")
	}
}
