package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The printed K:Prevent damage keyword family (engine/cards/kw_prevent.go):
// Fog Bank (combat damage to and by it), Guard Gomazoa (combat damage to it)
// and Dawn Elemental (all damage to it). kgen lowers each sentence to a
// CardDef fact, and the kernel's damage paths read it; gorge is the answer at
// every decision. The deck pair puts the carriers on one side and a 3/3
// attacker plus burn on the other, so the games exercise both directions.
func TestPreventKeywordLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set, err := cardset.RawByName("preventkw")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Fog Bank", "Guard Gomazoa", "Dawn Elemental"} {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the preventkw pool", name)
		}
		if set.Status(i) == cardset.StatusNotLowered {
			t.Fatalf("%s is not lowered (%s): the printed K:Prevent sentence is still refused", name, set.Reason(i))
		}
	}
	decks := [2][]cardset.Entry{
		{
			{Name: "Fog Bank", Count: 4}, {Name: "Guard Gomazoa", Count: 4}, {Name: "Dawn Elemental", Count: 4},
			{Name: "Plains", Count: 14}, {Name: "Island", Count: 14},
		},
		{
			{Name: "Hill Giant", Count: 10}, {Name: "Lightning Bolt", Count: 10}, {Name: "Shock", Count: 10},
			{Name: "Mountain", Count: 10},
		},
	}
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	prevented := 0
	finish := func(g *GorgeSide) {
		for i := range g.E.L.Events {
			ev := &g.E.L.Events[i]
			if ev.Kind != events.Note || !strings.Contains(ev.Text, "prevent") {
				continue
			}
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
				switch o.Face().Name {
				case "Fog Bank", "Guard Gomazoa", "Dawn Elemental":
					prevented++
				}
			}
		}
	}
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"preventkw-a", "preventkw-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
	}
	if prevented == 0 {
		t.Fatal("no prevention note was logged for a K:Prevent carrier: the fixture never exercised the keyword")
	}
}
