package rules

// cr-g57: CR 723.4 widens what the CONTROLLER of another player may see to
// what that player could see, but only while the control is live -- CR
// 723.1's "during that player's next turn". The grant is folded into
// state.Game.ControlledBy the moment Mindslaver's ability resolves, on the
// controller's own turn; before the fix view.controlsSeat read that map
// alone, so from the resolution on the controller's projection held the
// controlled seat's hand and every Secret event of that seat (its draws) was
// served whole to the controller, a full turn before the control began.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// activateMindslaverOn activates seat 0's Mindslaver (on the battlefield,
// {4} in the pool) targeting seat 1 and resolves it.
func activateMindslaverOn(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	submitChoices(t, e, abilityOption(t, e, id, 0).Index)
	for i := 0; i < 10 && e.G.Obj(id).Zone != state.ZGraveyard; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		switch d.Kind {
		case decision.KTarget:
			idx := indexOfPlayerOption(d, 1)
			if idx < 0 {
				t.Fatalf("Mindslaver target ask omitted seat 1: %+v", d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("target seat 1: %v", err)
			}
		case decision.KChoose:
			submitChoices(t, e, d.Options[0].Index)
		default:
			t.Fatalf("unexpected decision while activating Mindslaver: %+v", d)
		}
	}
	passUntilStackEmpty(t, e, 20)
	if ctl, ok := e.G.ControlledBy[1]; !ok || ctl != 0 {
		t.Fatalf("precondition: Mindslaver granted no control of seat 1 to seat 0: %+v", e.G.ControlledBy)
	}
}

// seatHandIn is seat p's PlayerView.Hand in v (nil = hidden).
func seatHandIn(t *testing.T, v view.View, p state.PlayerID) []view.CardView {
	t.Helper()
	for i := range v.Players {
		if v.Players[i].ID == p {
			return v.Players[i].Hand
		}
	}
	t.Fatalf("no seat %d in the view", p)
	return nil
}

// driveToTurnMin is driveToTurn answering every non-priority ask with its
// first d.Min options (a cleanup discard of two needs two), so a hand grown
// past seven by Vision Skeins does not stall the drive.
func driveToTurnMin(t *testing.T, e *Engine, turn int32, p state.PlayerID) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Over {
			t.Fatalf("game ended unexpectedly at turn %d", e.G.Turn)
		}
		if e.G.Turn >= turn && e.G.Active == p && e.G.Step == state.StepMain1 {
			return
		}
		d := e.Pending()
		if d == nil {
			continue
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		n := d.Min
		if n < 1 {
			n = 1
		}
		if len(d.Options) < n {
			t.Fatalf("decision with %d options needs %d: %+v", len(d.Options), n, d)
		}
		picks := make([]int, n)
		for j := range picks {
			picks[j] = d.Options[j].Index
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	t.Fatal("driveToTurnMin did not converge")
}

// TestMindslaverControlHidesTheHandUntilTheControlledTurn: seat 0 activates
// Mindslaver on its own turn 1 and then casts Vision Skeins (each player
// draws two). Until seat 1's turn 2 begins, seat 0 must not see seat 1's hand
// in its projection nor seat 1's two drawn cards in its event stream; during
// turn 2 it sees both (CR 723.4); from turn 3 on, neither again.
func TestMindslaverControlHidesTheHandUntilTheControlledTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	slaver := lookup(t, reg, "Mindslaver")
	skeins := lookup(t, reg, "Vision Skeins")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{slaver, skeins}, nil)
	id := moveByName(t, e, 0, "Mindslaver", state.ZBattlefield)
	moveByName(t, e, 0, "Vision Skeins", state.ZHand)
	if e.G.Active != 0 || e.G.Turn != 1 {
		t.Fatalf("precondition: seat 0's turn 1, got seat %d turn %d", e.G.Active, e.G.Turn)
	}
	addMana(t, e, 0, "CCCC")
	activateMindslaverOn(t, e, id)

	// The grant exists but is not live: seat 0's own turn 1.
	if h := seatHandIn(t, view.Project(e.G, e, 0, nil), 1); h != nil {
		t.Errorf("turn 1 (seat 0's own): seat 0's projection shows seat 1's hand %d cards before the controlled turn (CR 723.1/723.4)", len(h))
	}
	addMana(t, e, 0, "UU")
	mark := len(e.L.Events)
	castNamed(t, e, "Vision Skeins")
	passUntilStackEmpty(t, e, 20)
	draws := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind != events.Draw || ev.Player != 1 {
			continue
		}
		draws++
		if got := view.RedactEventFor(e.G, ev, 0, view.Seat); got.Obj != 0 {
			t.Errorf("turn 1: seat 0 is served seat 1's draw of Obj %d (%s) before the controlled turn: %+v", got.Obj, view.Describe(e.G, got), got)
		}
	}
	if draws != 2 {
		t.Fatalf("precondition: Vision Skeins drew %d cards for seat 1, want 2", draws)
	}

	// Seat 1's turn 2: the control is live, seat 0 sees the hand.
	driveToTurnMin(t, e, 2, 1)
	if h := seatHandIn(t, view.Project(e.G, e, 0, nil), 1); len(h) == 0 || len(h) != len(e.G.Zone(state.ZHand, 1)) {
		t.Fatalf("turn 2 (seat 1's, controlled): seat 0's projection shows %d of seat 1's %d hand cards, want all (CR 723.4)", len(h), len(e.G.Zone(state.ZHand, 1)))
	}
	// A third viewpoint is unchanged: seat 1 sees its own hand throughout.
	if h := seatHandIn(t, view.Project(e.G, e, 1, nil), 1); len(h) != len(e.G.Zone(state.ZHand, 1)) {
		t.Fatalf("turn 2: seat 1's own hand projected as %d cards", len(h))
	}

	// Seat 0's turn 3: the control has expired.
	driveToTurnMin(t, e, 3, 0)
	if h := seatHandIn(t, view.Project(e.G, e, 0, nil), 1); h != nil {
		t.Fatalf("turn 3: seat 0's projection still shows seat 1's hand after the control ended")
	}
	replayCheck(t, e, cfg)
}

// TestControlGrantedDuringTheControlledSeatsTurnWaitsForItsNextTurn: the
// grant armed DURING seat 1's own turn (Turn == ControlArmedTurn) is not live
// on that turn (CR 723.1 "that player's next turn"): the engine's redirect
// and the view agree. Both read state.Game.LiveController.
func TestControlGrantedDuringTheControlledSeatsTurnWaitsForItsNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, nil, nil)
	driveToTurn(t, e, 2, 1)
	e.emit(events.Event{Kind: events.ControlPlayerChange, Player: 0,
		IDs: []state.ObjID{state.PlayerRef(1)}, Amount: 1})
	if _, live := e.G.LiveController(1); live {
		t.Fatalf("a grant armed on seat 1's own turn %d is live on that turn", e.G.Turn)
	}
	if h := seatHandIn(t, view.Project(e.G, e, 0, nil), 1); h != nil {
		t.Errorf("seat 0 sees seat 1's hand on the turn the grant was armed")
	}
	if d := e.Pending(); d != nil && d.Player != 1 {
		t.Fatalf("seat 1's own-turn decision redirected to seat %d", d.Player)
	}
	driveToTurn(t, e, 4, 1)
	if ctl, live := e.G.LiveController(1); !live || ctl != 0 {
		t.Fatalf("turn %d: the grant is not live on seat 1's next turn", e.G.Turn)
	}
	if h := seatHandIn(t, view.Project(e.G, e, 0, nil), 1); len(h) != len(e.G.Zone(state.ZHand, 1)) {
		t.Fatalf("turn 4: seat 0's projection shows %d of seat 1's hand", len(h))
	}
}
