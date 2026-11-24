package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// K:Enlist (CR 702.154a), lowered from the printed keyword: as an enlist
// carrier attacks, its controller may tap a nonattacking creature they
// control without summoning sickness, and the attacker gets its power until
// end of turn. Played decision by decision against gorge over the enlist
// pool; the random policy takes both the decline and the yes, Argivian
// Cavalier's token and the plain creatures give the carriers something to
// tap, and Balduvian Berserker's dies trigger reads the pumped power.

var enlistDecks = [2][]cardset.Entry{
	{
		{Name: "Argivian Cavalier", Count: 4}, {Name: "Benalish Faithbonder", Count: 4},
		{Name: "Coalition Warbrute", Count: 4}, {Name: "Balduvian Berserker", Count: 4},
		{Name: "Yavimaya Steelcrusher", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Barkweave Crusher", Count: 4}, {Name: "Linebreaker Baloth", Count: 4},
		{Name: "Hexbane Tortoise", Count: 4}, {Name: "Coalition Skyknight", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	},
}

// enlistScan counts the enlist actions taken (gorge's Enlist event, one per
// yes) and the attack declarations (each a chance to enlist).
func enlistScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch {
		case ev.Kind == events.Enlist:
			n["enlist"]++
		case ev.Kind == events.DeclareAttackers && len(ev.IDs) > 0:
			n["declarations"]++
		}
	}
}

func TestEnlistLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "enlist", enlistDecks, 200, enlistScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"enlist", "declarations"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
}
