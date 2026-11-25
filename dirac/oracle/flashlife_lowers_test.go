package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Flashback with a mana part and a life part in lockstep against gorge (CR
// 702.34a, 601.2g-h): the "Flashback--{1}{C}, Pay 3 life" cycle (Deep
// Analysis, Acorn Harvest, Crippling Fatigue, Flash of Defiance, Spirit
// Flare), whose PayLife<3> kgen lowers as the flashback cast's FbCost beside
// its mana part (lower_fbcost.go). The expected answer at every decision is
// gorge's.
const flashLifePool = "flashlife"

var flashLifeDecks = [2][]cardset.Entry{
	{
		{Name: "Deep Analysis", Count: 4}, {Name: "Crippling Fatigue", Count: 4}, {Name: "Flash of Defiance", Count: 2},
		{Name: "Scathe Zombies", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Acorn Harvest", Count: 4}, {Name: "Spirit Flare", Count: 4},
		{Name: "Savannah Lions", Count: 7}, {Name: "Grizzly Bears", Count: 7},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
}

func TestFlashLifeLowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, flashLifePool, flashLifeDecks, 200, "KFLASHLIFE_SEEDS")
}

// TestFlashLifeDecksPlayable names a refused carrier here, not only by a
// stall far into the games.
func TestFlashLifeDecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, flashLifePool, flashLifeDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the %s pool", s, flashLifePool)
		}
	}
}

// TestFlashLifeMutation: with the life part dropped (the flashback cast
// charging only its mana), or the life part kept but the cast priced as one
// gorge's planner plans (no witness tapped before the target ask), some seed
// diverges, so the pool plays the mana-and-life flashback cast.
// It is not parallel: it sets the kernel's DebugFbLifePlanned.
func TestFlashLifeMutation(t *testing.T) {
	n := lockstepRun(t, flashLifePool, flashLifeDecks, 60, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].FbCost.Flags&kop.CostPayLife != 0 {
				cards[i].FbCost = kop.Cost{}
			}
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Error("no life part: no seed diverged: the pool never pays a flashback life part")
	}
	dirac.DebugFbLifePlanned = true
	defer func() { dirac.DebugFbLifePlanned = false }()
	if lockstepRun(t, flashLifePool, flashLifeDecks, 60, false, nil) == 0 {
		t.Error("planned: no seed diverged: the pool never lowers a mana-and-life flashback cast")
	}
}
