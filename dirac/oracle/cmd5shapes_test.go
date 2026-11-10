package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cmd5 trigger-shape lockstep: Angelic Skirmisher's keyword choice for
// the creatures its controller has, Hunter's Insight's delayed combat-damage
// draw to a player or planeswalker, Chandra's Phoenix returning from the
// graveyard on burn damage, and Vastwood Hydra's dies-trigger counter
// distribution, over a red-green and a red-white deck of the fixture pool.

var kcmd5shapesDecks = [2][]cardset.Entry{
	{
		{Name: "Chandra's Phoenix", Count: 3}, {Name: "Lightning Bolt", Count: 4}, {Name: "Vastwood Hydra", Count: 6},
		{Name: "Hunter's Insight", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Angelic Skirmisher", Count: 5}, {Name: "Chandra's Phoenix", Count: 3}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Chandra, Flameshaper", Count: 1}, {Name: "Gray Ogre", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 14}, {Name: "Mountain", Count: 6},
	},
}

func TestCmd5ShapesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cmd5shapes")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcmd5shapesDecks {
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
	tbl, _, _, err := set.Matchup(kcmd5shapesDecks[0], kcmd5shapesDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st dirac.EffEOTTrigStats
	dirac.DebugEffEOTTrigStats = &st
	defer func() { dirac.DebugEffEOTTrigStats = nil }()
	games, stalls, phoenixBack, distPicks, skirmChoices := 0, 0, 0, 0, 0
	// the asks both sides agree on: the keyword choice and the counter pick
	check := func(ref *GorgeSide, k Side) *Divergence {
		if d := ref.Pending(); d != nil {
			switch {
			case d.Prompt == "Choose a keyword":
				skirmChoices++
			case d.ResumeKind == "counter_dist":
				distPicks++
			}
		}
		return nil
	}
	for seed := uint64(1); seed <= 400; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"cmd5shapes-a", "cmd5shapes-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kinnerlightPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		games++
		for _, e := range g.E.L.Events {
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			switch {
			case e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZHand && o.Face().Name == "Chandra's Phoenix":
				phoenixBack++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d stalls; Phoenix returns %d, Hydra picks %d, Hunter's Insight draws %d, keyword-choice decisions %d", games, stalls, phoenixBack, distPicks, st.DmgFires, skirmChoices)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	// the preconditions: each shape was reached
	if phoenixBack < 3 || distPicks < 3 || st.DmgFires < 3 || skirmChoices < 3 {
		t.Errorf("shapes under-reached: Phoenix returns %d, Hydra picks %d, Hunter's Insight draws %d, keyword-choice decisions %d (want 3 each)", phoenixBack, distPicks, st.DmgFires, skirmChoices)
	}
}
