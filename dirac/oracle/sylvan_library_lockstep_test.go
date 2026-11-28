package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The sylvanlibrary fixture: Sylvan Library beside vanilla creatures and
// basics. At its controller's draw step the trigger offers the optional cost
// Draw<2/You>; paid, the controller chooses two cards in hand drawn this turn
// (Card.YouOwn+DrawnThisTurn) and, for each in the order they pick, pays 4
// life or puts the card on top of their library (gorge's RepeatEach with
// ChooseOrder$ True over the chosen cards, an UnlessCost$ PayLife<4> body).
var sylvanLibraryDecks = []fixtureDecks{
	{
		{{Name: "Sylvan Library", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Craw Wurm", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 8}},
		{{Name: "Sylvan Library", Count: 4}, {Name: "Grizzly Bears", Count: 10}, {Name: "Craw Wurm", Count: 6},
			{Name: "Forest", Count: 20}},
	},
}

var sylvanLibraryWatch = map[string]bool{"Sylvan Library": true}

// sylvanLibraryPolicy is fixtureWatchPolicy counting, from gorge's pending
// decision, the asks of a resolving Sylvan Library trigger: each per-card
// "pay 4 life" ask (one per pass of the RepeatEach loop over the chosen
// cards), the ChooseOrder$ ask before the loop, and the hand pick a pass
// poses while both chosen cards are still in hand.
func sylvanLibraryPolicy(seed uint64, casts map[string]int) Policy {
	inner := fixtureWatchPolicy(seed, sylvanLibraryWatch, casts)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		if d := c.G.Pending(); d != nil && sylvanLibrarySource(c, d) {
			switch d.ResumeKind {
			case "unless_pay":
				casts["loop pass"]++
			case "repeat_choose_order":
				casts["order ask"]++
			case "hand_move":
				casts["hand pick"]++
			}
		}
		return inner.Pick(c)
	})
}

func sylvanLibrarySource(c *PickCtx, d *decision.Decision) bool {
	o := c.G.E.G.Obj(d.Source)
	return o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Sylvan Library"
}

// TestSylvanLibraryLockstep plays the sylvanlibrary pool in lockstep with
// gorge. The precondition is that Sylvan Library was cast and its loop asked
// (a game resolved the chosen-card RepeatEach, ordered its subjects and picked
// a card to put back). The mutation makes DrawnThisTurn admit every hand card
// (dirac.DebugDrawnAnyHand), so the choice offers cards gorge does not.
func TestSylvanLibraryLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstepPolicy(t, "sylvanlibrary", sylvanLibraryDecks, 60, true, nil, sylvanLibraryPolicy)
		t.Logf("%d games, %d divergent, %d stalls; casts %v", run.games, run.divs, run.stalls, run.casts)
		if run.casts["Sylvan Library"] == 0 {
			t.Errorf("Sylvan Library was never cast")
		}
		for _, k := range []string{"loop pass", "order ask", "hand pick"} {
			if run.casts[k] == 0 {
				t.Errorf("no game posed Sylvan Library's %s", k)
			}
		}
	})
	t.Run("mutation-any-hand", func(t *testing.T) {
		dirac.DebugDrawnAnyHand = true
		defer func() { dirac.DebugDrawnAnyHand = false }()
		run := runFixtureLockstepPolicy(t, "sylvanlibrary", sylvanLibraryDecks, 60, false, nil, sylvanLibraryPolicy)
		run.requireDivergence(t, "DrawnThisTurn admitting every hand card")
	})
}
