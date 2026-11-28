package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// hiddenpick_chooseropponent_fourseat_test.go: a hidden graveyard pick an
// opponent answers (Chooser$ Opponent: Tasigur, the Golden Fang's "return a
// nonland card of an opponent's choice from your graveyard") plays four-seat
// games in lockstep. With two or more live opponents the controller chooses
// WHICH one answers (CR 608.2d; the CR 801.5b example), and that seat answers
// the pick.
var hiddenPickOpponentPodDeck = []cardset.Entry{
	{Name: "Tasigur, the Golden Fang", Count: 8},
	{Name: "Dark Ritual", Count: 8}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 8},
}

// TestFourSeatHiddenPickChooserOpponentLockstep: Tasigur is activated whenever
// offered; the controller always names
// its second opponent in AliveFrom order, never the next live seat, and that
// opponent answers the pick that follows. The mutation (DebugSearchOppNext)
// has the next live seat answer whatever was picked and must diverge.
func TestFourSeatHiddenPickChooserOpponentLockstep(t *testing.T) {
	corpusDirOrSkip(t)
	var chosen, asks, picks int
	oldSteer := podClassSteer
	defer func() { podClassSteer = oldSteer }()
	podClassSteer = func(c *PickCtx) (Canon, bool) {
		if c.G == nil || c.G.fr == nil {
			return 0, false
		}
		d := c.G.fr.d
		if d == nil || d.ResumeKind != "hidden_pick_opponent" || len(d.Options) < 2 {
			return tasigurSteer(c)
		}
		want := RefPlayer + Ref(d.Options[1].Player)
		for _, a := range c.D.Options {
			if a.Tgt() == want {
				return a, true
			}
		}
		return 0, false
	}
	watch := func(c *PickCtx) bool {
		if c.G == nil || c.G.fr == nil {
			return false
		}
		d := c.G.fr.d
		if d == nil {
			return false
		}
		switch d.ResumeKind {
		case "hidden_pick_opponent":
			if len(d.Options) < 2 {
				return false
			}
			chosen = int(d.Options[1].Player)
			asks++
			return true
		case "hidden_pick":
			if effects.ChangeZoneOf(d.ResumeSA).Chooser != "Opponent" || liveSeats(c) < 3 {
				return false
			}
			if int(d.Player) != chosen {
				t.Errorf("the hidden pick is posed to seat %d, want the chosen opponent %d", d.Player, chosen)
			}
			picks++
			return true
		}
		return false
	}
	podClassLockstep(t, hiddenPickOpponentPodDeck, 24, 10, watch, &dirac.DebugSearchOppNext)
	if asks < 5 || picks < 5 {
		t.Fatalf("the run watched too few opponent choices (asks %d, picks %d)", asks, picks)
	}
	t.Logf("asks %d, picks %d", asks, picks)
}

// tasigurSteer casts Tasigur and takes its activation whenever offered.
// A delve cast is offered only against the floating pool (mana abilities are
// never options), so with Tasigur in the deciding seat's hand on its own main
// phase and the stack empty it casts Dark Ritual first.
func tasigurSteer(c *PickCtx) (Canon, bool) {
	const tasigur = "Tasigur, the Golden Fang"
	if a, ok := findAction(c, func(a Canon) bool {
		return (a.Kind() == ActActivate || a.Kind() == ActCast) && c.G.M.InstanceCard(int(a.Src())) == tasigur
	}); ok {
		return a, true
	}
	g, me := c.G.E.G, c.G.fr.d.Player
	if g.Active != me || !g.Step.IsMain() || len(g.Stack) != 0 {
		return 0, false
	}
	if !slices.ContainsFunc(g.Zone(state.ZHand, me), func(id state.ObjID) bool { return g.Obj(id).Face().Name == tasigur }) {
		return 0, false
	}
	return findAction(c, func(a Canon) bool {
		return a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) == "Dark Ritual"
	})
}
