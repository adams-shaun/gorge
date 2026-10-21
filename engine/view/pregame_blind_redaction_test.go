package view_test

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestPregameBlindRedactsOwnSecretDraws is the redaction half of E3-fx3
// (CR 103.5): while the CR 103.1 starting-player choice is pending, the
// opening-hand deal reaches the log as each seat's own Secret Draw events,
// which rule (1) of the redactor passes through for the owner
// (engine/view/redact.go). view.RedactEventForPregameBlind is the serve-time arm
// the host's seat-scoped serving paths set from the engine's
// StartingChoicePending probe: the owner's own Secret draw is redacted to
// exactly the shape an opponent gets -- every payload field zeroed, the
// event's shape kept -- so no opening-hand card id leaves the server during
// the window. The omniscient spectator arm is unchanged, and a non-Secret
// event is untouched by the blind. The recorded log, the event kinds and
// the chain heads are not inputs here: this is only what a seat is SERVED.
func TestPregameBlindRedactsOwnSecretDraws(t *testing.T) {
	e := rules.NewStartingPlayerChoice(rules.Config{
		Seed: 1, Names: []string{"a", "b"}, Decks: mountainDecks(t, 2),
	})
	if !e.StartingChoicePending() {
		t.Fatalf("fixture precondition: the CR 103.1 choice is not pending")
	}
	var draws []events.Event
	for i := range e.L.Events {
		if ev := e.L.Events[i]; ev.Kind == events.Draw && ev.Secret && ev.Player == 0 {
			draws = append(draws, ev)
		}
	}
	if len(draws) != 7 {
		t.Fatalf("fixture precondition: %d seat-0 genesis Secret draws, want 7", len(draws))
	}
	if draws[0].Obj == 0 {
		t.Fatalf("fixture precondition: the genesis draw carries no card id")
	}
	for _, draw := range draws {
		// Control: without the blind, today's redaction hands the owner
		// their own card id back. If this ever stops holding, the blind arm
		// below is no longer hiding anything the plain path revealed.
		if open := view.RedactEventFor(e.G, draw, 0, view.Seat); open.Obj != draw.Obj {
			t.Fatalf("control: owner-visible redaction no longer passes the own draw through (Obj %d -> %d)", draw.Obj, open.Obj)
		}
		// The blind: shape-only, exactly the opponent's arm.
		blind := view.RedactEventForPregameBlind(e.G, draw, 0, view.Seat)
		if blind.Obj != 0 || blind.Amount != 0 || blind.Counter != "" || blind.Text != "" ||
			len(blind.IDs) != 0 || len(blind.Pairs) != 0 {
			t.Errorf("blind draw carried payload: %+v", blind)
		}
		if blind.Kind != events.Draw || blind.Player != 0 || blind.From != state.ZLibrary ||
			blind.To != state.ZHand || !blind.Secret || blind.Seq != draw.Seq {
			t.Errorf("blind draw lost its shape: %+v", blind)
		}
		// The omniscient spectator is unchanged: the blind never fires off
		// the Seat path, and their visibility already includes every hand.
		if omni := view.RedactEventForPregameBlind(e.G, draw, view.NoSeat, view.Omniscient); omni.Obj != draw.Obj {
			t.Errorf("omniscient spectator lost the draw's Obj %d", draw.Obj)
		}
	}
	// A non-Secret event is untouched by the blind: the arm keys on Secret.
	// DeepEqual, not ==: a redacted event's IDs/Pairs are slices, and a
	// non-Secret genesis Shuffle carries them.
	for i := range e.L.Events {
		ev := e.L.Events[i]
		if ev.Secret {
			continue
		}
		if got, want := view.RedactEventForPregameBlind(e.G, ev, 0, view.Seat), view.RedactEventFor(e.G, ev, 0, view.Seat); !reflect.DeepEqual(got, want) {
			t.Errorf("non-Secret event seq %d differs under the blind: %+v vs %+v", ev.Seq, got, want)
		}
		break
	}
}

// TestPregameBlindRedactsControlledSeatSecrets pins the CR 720.4 half of
// the blind arm: a Secret event whose Player is a seat the viewer CONTROLS
// (not their own) is redacted too while the blind is set, the same widening
// rule (1) grants the plain redactor. A hand-built game with a
// ControlledBy entry is the fixture -- the pregame window itself never has
// live control, so only the unit shape can pin this branch.
func TestPregameBlindRedactsControlledSeatSecrets(t *testing.T) {
	e := rules.NewStartingPlayerChoice(rules.Config{
		Seed: 1, Names: []string{"a", "b"}, Decks: mountainDecks(t, 2),
	})
	var draw events.Event
	for i := range e.L.Events {
		if ev := e.L.Events[i]; ev.Kind == events.Draw && ev.Secret && ev.Player == 1 {
			draw = ev
			break
		}
	}
	if draw.Obj == 0 {
		t.Fatalf("fixture precondition: no seat-1 genesis Secret draw with a card id")
	}
	// Precondition for the widening branch: seat 1's owner is controlled by
	// viewer 0 in this hand-built state, so the plain redactor passes the
	// secret to the controller unchanged -- and the blind must strip it.
	g := e.G
	g.ControlledBy = map[state.PlayerID]state.PlayerID{1: 0}
	g.Active, g.Turn = 1, 1 // cr-g57: the grant (armed turn 0) is live only during seat 1's turn
	if open := view.RedactEventFor(e.G, draw, 0, view.Seat); open.Obj != draw.Obj {
		t.Fatalf("control: controller-visible redaction no longer passes the controlled seat's secret through (Obj %d -> %d)", draw.Obj, open.Obj)
	}
	if blind := view.RedactEventForPregameBlind(e.G, draw, 0, view.Seat); blind.Obj != 0 {
		t.Errorf("blind left the controlled seat's secret payload: %+v", blind)
	}
}
