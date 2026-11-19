package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Path of Ancestry as a payment source with its scry rider, in two-seat
// Commander games: its {T} ability adds one mana of a colour in the
// commander's identity (CR 903.4) and, when that mana is spent on a creature
// spell sharing a creature type with a commander, scries 1. Both engines offer
// and pay with a plan that taps the Path, and queue the scry after the cast.
// Each seat's commander is Rograkh (a Kobold Warrior, red), beside Warrior
// creatures that share a type and a Gray Ogre that does not. Each deck holds
// one Path (Commander is singleton): the kernel records one observed source per
// cast, so a payment tapping two Paths stalls loudly (obssrc).
var pathScryDecks = [2][]cardset.Entry{
	{
		{Name: "Rograkh, Son of Rohgahh", Count: 1}, {Name: "Path of Ancestry", Count: 1}, {Name: "Viashino Slasher", Count: 10},
		{Name: "Gray Ogre", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Rograkh, Son of Rohgahh", Count: 1}, {Name: "Path of Ancestry", Count: 1}, {Name: "Viashino Slasher", Count: 7},
		{Name: "Gray Ogre", Count: 7}, {Name: "Shock", Count: 3}, {Name: "Mountain", Count: 12},
	},
}

// pathScryRun plays seeds 1..n of the duel in lockstep. It returns the number
// of scries gorge resolved off a Path of Ancestry's rider and the first
// divergence's seed (0 none).
func pathScryRun(t *testing.T, n uint64) (scries int, div uint64) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ancestryscry")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range pathScryDecks {
		for _, e := range pathScryDecks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(pathScryDecks[0], pathScryDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	names := [2]string{"ancestryscry-a", "ancestryscry-b"}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, names, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.Commander = true
		m.Commanders, m.Partners = [2]int{0, 0}, [2]int{-1, -1}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		scries += pathScries(g)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[max(d.At-14, 0):d.At]))
			t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return scries, seed
		}
	}
	return scries, 0
}

// pathScries counts the scries gorge's log holds whose source is a Path of
// Ancestry: the rider fired off mana the Path produced.
func pathScries(g *GorgeSide) int {
	n := 0
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.Scry {
			continue
		}
		if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Path of Ancestry" {
			n++
		}
	}
	return n
}

// TestPathOfAncestryScryLockstep: the Path pays creature spells and its scry
// plays in lockstep; with the kernel's spent-mana reader never matching
// (dirac.SetDebugSonManaMutate), or its commander-type filter answering the
// opposite (dirac.DebugSharesCrCmdrFlip), the same games diverge, so the
// agreement depends on the rider.
func TestPathOfAncestryScryLockstep(t *testing.T) {
	const seeds = 100
	scries, div := pathScryRun(t, seeds)
	t.Logf("%d Path of Ancestry scries over %d games", scries, seeds)
	if div != 0 {
		t.Fatalf("seed %d diverged", div)
	}
	if scries == 0 {
		t.Fatal("no game resolved a Path of Ancestry scry: the lockstep never reaches the rider")
	}
	dirac.SetDebugSonManaMutate(1)
	defer dirac.SetDebugSonManaMutate(0)
	if _, div := pathScryRun(t, seeds); div == 0 {
		t.Fatal("the kernel's spent-mana reader never matching still agrees with gorge")
	}
	dirac.SetDebugSonManaMutate(0)
	dirac.DebugSharesCrCmdrFlip = true
	defer func() { dirac.DebugSharesCrCmdrFlip = false }()
	if _, div := pathScryRun(t, seeds); div == 0 {
		t.Fatal("the kernel's commander-type filter answering the opposite still agrees with gorge")
	}
}
