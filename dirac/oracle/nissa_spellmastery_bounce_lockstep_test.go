package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nissa's Pilgrimage with spell mastery live (the cast Pilgrimages fill the
// graveyard) finds up to three basic Forests: one enters the battlefield and
// DBHand's Defined$ Remembered moves every other remembered card to hand,
// then the library shuffles once. The kernel's Bounce obj=Remembered moved
// only the first remembered card; six Forests keep three-pick resolutions
// reachable.
var knissabounceDecks = [2][]cardset.Entry{{
	{Name: "Nissa's Pilgrimage", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 14},
	{Name: "Llanowar Elves", Count: 5}, {Name: "Elvish Mystic", Count: 5},
}, {
	{Name: "Mountain", Count: 20}, {Name: "Lightning Bolt", Count: 10}, {Name: "Shock", Count: 10},
}}

// knissabounceScan counts gorge's Nissa's Pilgrimage resolutions that moved
// one library card to the battlefield and two to hand (a three-pick).
func knissabounceScan(g *GorgeSide) (three int) {
	if g == nil {
		return 0
	}
	evs := g.E.L.Events
	for i := range evs {
		if evs[i].Kind != events.Resolve || sourceCardName(g.E.G, evs[i].Obj) != "Nissa's Pilgrimage" {
			continue
		}
		bf, hand := 0, 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind != events.MoveZone || evs[j].From != state.ZLibrary {
				continue
			}
			switch evs[j].To {
			case state.ZBattlefield:
				bf++
			case state.ZHand:
				hand++
			}
		}
		if bf == 1 && hand == 2 {
			three++
		}
	}
	return three
}

func TestNissaSpellMasteryBounceLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, knissabounceDecks)
	tbl, _, _, err := set.Matchup(knissabounceDecks[0], knissabounceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played := map[string]int{}
	three := 0
	for seed := uint64(1); seed <= 384; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"knb-a", "knb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		three += knissabounceScan(gs)
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("three-pick Nissa's Pilgrimage resolutions %d", three)
	if three < 10 {
		t.Fatalf("precondition: %d three-pick resolutions, want at least 10", three)
	}
}
