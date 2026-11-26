package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// S:Mode$ OptionalAttackCost (CR 702.100a "you may exert CARDNAME as it
// attacks. When you do, ..."), lowered as the kernel's exert offer plus a
// self-Exerted rider trigger. Played decision by decision against gorge over
// the exert pool; the random policy takes both the decline and the yes.

var exertDecks = [2][]cardset.Entry{
	{
		{Name: "Khenra Scrapper", Count: 4}, {Name: "Bitterblade Warrior", Count: 4},
		{Name: "Glory-Bound Initiate", Count: 4}, {Name: "Champion of Rhonas", Count: 4},
		{Name: "Hooded Brawler", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Tah-Crop Elite", Count: 4}, {Name: "Gust Walker", Count: 4},
		{Name: "Themberchaud", Count: 4}, {Name: "Oketra's Avenger", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	},
}

// exertScan counts the exerts taken (gorge's Exert event, one per yes) and the
// attack declarations (each a chance to exert).
func exertScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch {
		case ev.Kind == events.Exert && ev.Amount >= 0:
			n["exert"]++
		case ev.Kind == events.DeclareAttackers && len(ev.IDs) > 0:
			n["declarations"]++
		}
	}
}

func TestExertLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "exert", exertDecks, 200, exertScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"exert", "declarations"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
}
