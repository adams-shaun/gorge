package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// departedEngine is a three-seat corpus game (seat 0 starts) with Reanimate
// in seat 0's deck, Grizzly Bears in seat 1's and Act of Treason in seat
// 2's, the rest Mountains, driven to seat 0's turn-1 Main1. Seat 0 has
// already Reanimated seat 1's Bears, so seat 0 controls a permanent seat 1
// owns with no control-changing effect involved. It returns the Bears.
func departedEngine(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	m := lookup(t, reg, "Mountain")
	fill := func(extras ...*cards.Card) []*cards.Card {
		deck := append([]*cards.Card{}, extras...)
		for len(deck) < 40 {
			deck = append(deck, m)
		}
		return deck
	}
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{fill(lookup(t, reg, "Reanimate")), fill(lookup(t, reg, "Grizzly Bears")), fill(lookup(t, reg, "Act of Treason"))}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	bears := moveByName(t, e, 1, "Grizzly Bears", state.ZGraveyard)
	reanimate := moveByName(t, e, 0, "Reanimate", state.ZHand)
	addMana(t, e, 0, "B")
	castAt(t, e, reanimate, bears)
	if o := e.G.Obj(bears); o.Zone != state.ZBattlefield || o.Controller != 0 || o.Owner != 1 {
		t.Fatalf("CR 800.4a seq %d: precondition: Bears zone %v controller %d owner %d, want battlefield under seat 0, owned by seat 1",
			len(e.L.Events), o.Zone, o.Controller, o.Owner)
	}
	return e, cfg, bears
}

// concedeSeat submits the concede option on the pending priority decision,
// which must be p's.
func concedeSeat(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != p {
		t.Fatalf("seq %d: want seat %d's priority decision to concede on, got %+v", len(e.L.Events), p, d)
	}
	submitChoices(t, e, highestPriorityOptionWithKind(t, e, "concede").Index)
}

// TestCR800_4aDepartedPlayersControlledObjectIsExiled: CR 800.4a, "Then, if
// there are any objects still controlled by that player, those objects are
// exiled." Seat 0 Reanimates seat 1's Grizzly Bears and concedes; the Bears
// (owned by seat 1, still in the game) are exiled, not left on the
// battlefield under a departed seat.
func TestCR800_4aDepartedPlayersControlledObjectIsExiled(t *testing.T) {
	t.Parallel()
	e, cfg, bears := departedEngine(t)
	concedeSeat(t, e, 0)
	if !e.G.Players[0].Lost || e.G.Over {
		t.Fatalf("CR 800.4a seq %d: seat 0 lost=%v over=%v, want seat 0 out and the game going on", len(e.L.Events), e.G.Players[0].Lost, e.G.Over)
	}
	passUntilStackEmpty(t, e, 10)
	if o := e.G.Obj(bears); o.Zone != state.ZExile {
		t.Fatalf("CR 800.4a seq %d: Bears controlled by departed seat 0: zone %v controller %d, want exile", len(e.L.Events), o.Zone, o.Controller)
	}
	replayCheck(t, e, cfg)
}

// TestCR800_4cEndedControlEffectExilesWhenDefaultControllerLeft: CR 800.4c,
// "If an effect that gives a player still in the game control of an object
// ends, there is no other effect giving control of that object to another
// player in the game, and the player who controlled that object by default
// has left the game, the object is exiled." Seat 0 Reanimates seat 1's
// Bears; on turn 3 seat 2 casts Act of Treason on them; seat 0 concedes
// (the Bears are seat 2's, so 800.4a leaves them); at seat 2's cleanup the
// Act ends and the Bears are exiled, not handed to their owner.
func TestCR800_4cEndedControlEffectExilesWhenDefaultControllerLeft(t *testing.T) {
	t.Parallel()
	e, cfg, bears := departedEngine(t)
	driveToStepAll(t, e, 3, 2, state.StepMain1)
	act := moveByName(t, e, 2, "Act of Treason", state.ZHand)
	addMana(t, e, 2, "RCC")
	castAt(t, e, act, bears)
	if o := e.G.Obj(bears); o.Controller != 2 {
		t.Fatalf("CR 800.4c seq %d: precondition: Act of Treason left the Bears with seat %d, want seat 2", len(e.L.Events), o.Controller)
	}
	passOnce(t, e) // seat 2 passes; seat 0 receives priority
	concedeSeat(t, e, 0)
	if o := e.G.Obj(bears); o.Zone != state.ZBattlefield || o.Controller != 2 {
		t.Fatalf("CR 800.4c seq %d: after seat 0 left: Bears zone %v controller %d, want still seat 2's on the battlefield", len(e.L.Events), o.Zone, o.Controller)
	}
	driveToStepAll(t, e, 4, 1, state.StepUpkeep)
	if o := e.G.Obj(bears); o.Zone != state.ZExile {
		t.Fatalf("CR 800.4c seq %d: Act of Treason ended with default controller seat 0 gone: Bears zone %v controller %d, want exile", len(e.L.Events), o.Zone, o.Controller)
	}
	replayCheck(t, e, cfg)
}
