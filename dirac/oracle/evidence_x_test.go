package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The variable collect-evidence lockstep (CR 701.59a): Incinerator of the
// Guilty announces X (gorge triggeredCostXAsk, "trigger_cost_x") and collects
// evidence X as a trigger's own cost, then deals X to each creature and
// planeswalker the damaged player controls; Lamplight Phoenix exiles itself
// (the reserved component pick) and collects evidence 4 from the rest of the
// graveyard, then returns tapped; Urgent Necropsy collects evidence equal to
// its targets' total mana value (the amount settles once the targets exist, a
// graveyard short of it aborts the cast). The burn and removal in both decks fill the
// graveyards. The expected answer at every decision is gorge's.
var kcollectevidencexMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Incinerator of the Guilty", Count: 4}, {Name: "Lamplight Phoenix", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 20},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}, {
	{
		{Name: "Urgent Necropsy", Count: 6}, {Name: "Murder", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 4},
		{Name: "Walking Corpse", Count: 2}, {Name: "Swamp", Count: 11}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Juggernaut", Count: 3}, {Name: "Pilgrim's Eye", Count: 3}, {Name: "Walking Corpse", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}}

// kcollectevidencexTally counts, on gorge's side, Incinerator's X
// announcements (and those whose top X is 3 or more) and the evidence picks
// posed for each carrier.
type kcollectevidencexTally struct{ games, xAsks, xTop3, incPick, phxPick, necPick, necPick5 int }

func kcollectevidencexCheck(tal *kcollectevidencexTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var last *decision.Decision
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, last = ref, nil
			tal.games++
		}
		d := ref.E.Pending()
		if d == nil || d == last {
			return nil
		}
		last = d
		if d.Kind != decision.KChoose || len(d.Options) == 0 {
			return nil
		}
		g := ref.E.G
		switch d.Options[0].Kind {
		case "trigger_cost_x":
			if chName(g, d.Source) == "Incinerator of the Guilty" {
				tal.xAsks++
				if d.Options[len(d.Options)-1].Amount >= 3 {
					tal.xTop3++
				}
			}
		case "evidence":
			switch chName(g, d.Source) {
			case "Incinerator of the Guilty":
				tal.incPick++
			case "Lamplight Phoenix":
				tal.phxPick++
			case "Urgent Necropsy":
				tal.necPick++
				if f := strings.Fields(d.Prompt); len(f) > 6 {
					if n, _ := strconv.Atoi(f[6]); n >= 5 {
						tal.necPick5++
					}
				}
			}
		}
		return nil
	}
}

func TestCollectEvidenceXLockstep(t *testing.T) {
	t.Parallel()
	var tal kcollectevidencexTally
	sonRun(t, "collectevidencex", kcollectevidencexMatchups, 256, true, nil, kcollectevidencexCheck(&tal))
	t.Logf("%d games: %d Incinerator X announcements (%d with a top X >= 3), %d Incinerator, %d Phoenix and %d Necropsy (%d owing 5 or more) evidence picks",
		tal.games, tal.xAsks, tal.xTop3, tal.incPick, tal.phxPick, tal.necPick, tal.necPick5)
	if tal.xAsks < 10 || tal.xTop3 < 3 || tal.incPick < 3 || tal.phxPick < 3 || tal.necPick < 3 || tal.necPick5 < 1 {
		t.Errorf("the pool barely pays: X asks %d (want >= 10), top X >= 3 %d (want >= 3), Incinerator picks %d (want >= 3), Phoenix picks %d (want >= 3), Necropsy picks %d (want >= 3), owing 5 or more %d (want >= 1)",
			tal.xAsks, tal.xTop3, tal.incPick, tal.phxPick, tal.necPick, tal.necPick5)
	}
}
