package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy of a graveyard card (Clone ChoiceZone$ Graveyard, kgen
// lower_cloneenter.go, kop.BecomeCopyGrave), in lockstep over the
// setsobjects2 fixture pool: Body Double, Voq, Torchbearer, Activated Sleeper
// and Invasion of Amonkhet's back face, Lazotep Convert. gorge poses the
// clone ask over every living player's graveyard (rules/cast_etbchoice.go
// etbCloneOptions with ChoiceZoneKind) and copies the picked card as printed
// if it is still there as the permanent enters (effects/clone.go
// cloneETBTemplateLegal).
//
// The expected answer at every decision is gorge's.

var so2Opp = []cardset.Entry{{Name: "Grizzly Bears", Count: 14}, {Name: "Shock", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10}}

var so2Matchups = []soMatchup{
	{card: "Body Double", deck: []cardset.Entry{
		{Name: "Body Double", Count: 8}, {Name: "Voq, Torchbearer", Count: 6}, {Name: "Walking Corpse", Count: 8},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	}, opp: so2Opp},
	{card: "Activated Sleeper", deck: []cardset.Entry{
		{Name: "Activated Sleeper", Count: 10}, {Name: "Walking Corpse", Count: 12},
		{Name: "Swamp", Count: 18},
	}, opp: so2Opp},
	{card: "Invasion of Amonkhet", deck: []cardset.Entry{
		{Name: "Invasion of Amonkhet", Count: 10}, {Name: "Walking Corpse", Count: 12},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	}, opp: so2Opp},
}

// so2Scan counts, per carrier (its printed front face), the as-enters copies
// gorge made of a card then in a graveyard.
func so2Scan(g *GorgeSide) map[string]int {
	r := map[string]int{}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.ClonePermanent || len(ev.IDs) == 0 || ev.IDs[0] == 0 {
			continue
		}
		o, tpl := g.E.G.Obj(ev.Obj), g.E.G.Obj(ev.IDs[0])
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 || tpl == nil {
			continue
		}
		if tpl.Zone == state.ZGraveyard || tpl.Zone == state.ZExile || tpl.Zone == state.ZLibrary || tpl.Zone == state.ZHand {
			r[o.Card.Faces[0].Name]++
		}
	}
	return r
}

func TestSetsObjects2Lockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("setsobjects2")
	if err != nil {
		t.Fatalf("setsobjects2 tables: %v (run make dirac-tables)", err)
	}
	for _, mu := range so2Matchups {
		t.Run(strings.NewReplacer(" ", "", ",", "", "'", "").Replace(mu.card), func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{mu.deck, mu.opp}
			var gd, sides [2][]*cards.Card
			for s := range decks {
				gd[s] = expandEntries(t, reg, decks[s])
				if len(gd[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(gd[s]))
				}
			}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: the pool's table: %v", mu.card, err)
			}
			tot := map[string]int{}
			for seed := uint64(1); seed <= 30; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchupSide(reg, [2]string{"so2-" + mu.card + "-a", "so2-" + mu.card + "-b"}, gd, sides, 1)
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
				pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts and activates whenever it can
				res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("%s seed %d: kernel stalled: %s", mu.card, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.card, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				for n, c := range so2Scan(g) {
					tot[n] += c
				}
			}
			t.Logf("%s: copies of an off-battlefield card per carrier %v", mu.card, tot)
			var bad bool
			switch mu.card {
			case "Body Double":
				bad = tot["Body Double"] < 3 || tot["Voq, Torchbearer"] < 3
			case "Activated Sleeper":
				bad = tot["Activated Sleeper"] < 3
			case "Invasion of Amonkhet":
				bad = tot["Invasion of Amonkhet"] < 1
			}
			if bad {
				t.Errorf("reach %v: the scenario the lowering must hold was not reached", tot)
			}
		})
	}
}
