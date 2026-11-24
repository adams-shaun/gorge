package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nissa's Pilgrimage with few Forests in the library: a resolution that finds
// exactly one Forest puts it onto the battlefield and leaves DBHand's Defined$
// Remembered empty. The search still happened, so its deferred "Then shuffle"
// is paid once (CR 701.24a; gorge zone_search.go emptyDefinedFetch, the
// kernel's emptyFetchShuffle). The lockstep compares the kernel to gorge on
// those games; the gorge log must show the one-Forest resolutions shuffling.
// Three Forests keep every search at two picks or fewer: casting the spell
// needs a Forest out of the library, so spell mastery's third pick never
// exists (a three-pick leaves two remembered cards for DBHand, a separate
// kernel shape this test does not cover).
var kemptylegDecks = [2][]cardset.Entry{{
	{Name: "Nissa's Pilgrimage", Count: 10}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 15},
	{Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Llanowar Elves", Count: 4},
}, {
	{Name: "Mountain", Count: 20}, {Name: "Lightning Bolt", Count: 10}, {Name: "Shock", Count: 10},
}}

// kemptylegScan counts gorge's Nissa's Pilgrimage resolutions that moved one
// library card (to the battlefield, none to hand) and those that also shuffled.
func kemptylegScan(g *GorgeSide) (one, shuffled int) {
	if g == nil {
		return 0, 0
	}
	evs := g.E.L.Events
	for i := range evs {
		if evs[i].Kind != events.Resolve || sourceCardName(g.E.G, evs[i].Obj) != "Nissa's Pilgrimage" {
			continue
		}
		bf, hand, shuf := 0, 0, 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			switch {
			case evs[j].Kind == events.MoveZone && evs[j].From == state.ZLibrary && evs[j].To == state.ZBattlefield:
				bf++
			case evs[j].Kind == events.MoveZone && evs[j].From == state.ZLibrary && evs[j].To == state.ZHand:
				hand++
			case evs[j].Kind == events.Shuffle:
				shuf++
			}
		}
		if bf == 1 && hand == 0 {
			one++
			if shuf == 1 {
				shuffled++
			}
		}
	}
	return one, shuffled
}

func TestSearchShuffleEmptyLegLockstep(t *testing.T) {
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
	decks := kcastDeck(t, reg, kemptylegDecks)
	tbl, _, _, err := set.Matchup(kemptylegDecks[0], kemptylegDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played := map[string]int{}
	one, shuffled := 0, 0
	for seed := uint64(1); seed <= 96; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kel-a", "kel-b"}, decks, 1)
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
		o, s := kemptylegScan(gs)
		one += o
		shuffled += s
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("one-Forest Nissa's Pilgrimage resolutions %d, shuffled %d", one, shuffled)
	if one < 10 {
		t.Fatalf("precondition: %d one-Forest Nissa's Pilgrimage resolutions, want at least 10", one)
	}
	if shuffled != one {
		t.Fatalf("%d of %d one-Forest Nissa's Pilgrimage resolutions shuffled, want all", shuffled, one)
	}
}
