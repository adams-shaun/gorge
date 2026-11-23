package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// jailerPodBoard is linkBoard over three seats: seat 0's Palace Jailer and
// seat 1's Grizzly Bears on the battlefield, seat 2 with nothing, parked on
// seat 0's first main phase.
func jailerPodBoard(t *testing.T) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	named := [][]*cards.Card{{mustCorpusCard(t, reg, "Palace Jailer")}, {mustCorpusCard(t, reg, "Grizzly Bears")}, nil}
	decks := make([][]*cards.Card, len(named))
	for i, d := range named {
		decks[i] = append(append([]*cards.Card{}, d...), mountainDeck(t, 40-len(d))...)
	}
	cfg := seatZeroStart(Config{Seed: 47, Names: []string{"a", "b", "c"}, Decks: decks})
	e := New(cfg)
	if e.choosing == chooseOpening {
		e.choosing = chooseNone
	}
	for seat, d := range named {
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			for _, c := range d {
				if o.Owner == state.PlayerID(seat) && o.Card == c {
					e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
					break
				}
			}
		}
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	return e
}

// TestPalaceJailerExileSkipsAfterOpponentCrowned is CR 610.3b over three
// seats: seat 2, an opponent of the Jailer's controller but not the target's
// controller, becomes the monarch after the exile trigger went on the stack,
// so the "until" event has already happened and the Bears stay on the
// battlefield with no return armed. The control takes the crown with the
// Jailer's own controller, which is no opponent: the Bears are exiled.
func TestPalaceJailerExileSkipsAfterOpponentCrowned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		crowned    state.PlayerID
		wantZone   state.Zone
		wantDelays int
	}{
		{"a third-seat opponent takes the crown", 2, state.ZBattlefield, 0},
		{"the jailer's controller takes the crown", 0, state.ZExile, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := jailerPodBoard(t)
			victim := jailerExileOnStack(t, e)
			top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
			e.emit(events.Event{Kind: events.MonarchChange, Player: tc.crowned})
			if !e.G.IsMonarch(tc.crowned) || e.G.Players[tc.crowned].BecameMonarchSeq <= top.PushMonarchSeq {
				t.Fatalf("precondition: seat %d must take the crown after the push (became %d, push %d)",
					tc.crowned, e.G.Players[tc.crowned].BecameMonarchSeq, top.PushMonarchSeq)
			}
			e.resolveTop()
			if got := e.G.Obj(victim).Zone; got != tc.wantZone {
				t.Fatalf("victim zone = %s, want %s", got, tc.wantZone)
			}
			if n := countBecomeMonarchDelayed(e); n != tc.wantDelays {
				t.Fatalf("BecomeMonarch registrations = %d (%+v), want %d", n, e.G.Delayed, tc.wantDelays)
			}
		})
	}
}
