package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStartYourEnginesSpeedStartsWithControlChange is kw:Start your engines'
// CR 702.179a control-change leaf, using the real corpus Amonkhet Raceway. The
// rule is a state-based action, so a speed-less seat that comes to control a
// permanent with the keyword must go to speed 1 even when the permanent was
// already on the battlefield and merely changed controllers -- the case a
// battlefield-entry-only hook cannot see.
func TestStartYourEnginesSpeedStartsWithControlChange(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := corpusEngineThree(t, reg,
		nil,
		[]*cards.Card{lookup(t, reg, "Amonkhet Raceway")},
		nil)
	// Precondition: seat 0 has no speed and controls no carrier before the
	// transfer, so a passing test cannot be a vacuous read of an
	// already-started seat.
	if got := e.G.Players[0].Speed; got != 0 {
		t.Fatalf("precondition: seat 0 speed before the transfer: %d, want 0", got)
	}
	id := moveByName(t, e, 1, "Amonkhet Raceway", state.ZBattlefield)
	// Precondition: the entry path starts seat 1, and leaves seat 0 alone --
	// the two compared values really differ.
	if got := e.G.Players[1].Speed; got != 1 {
		t.Fatalf("precondition: seat 1 speed after its raceway entered: %d, want 1", got)
	}
	if got := e.G.Players[0].Speed; got != 0 {
		t.Fatalf("precondition: seat 0 speed while seat 1 controls the raceway: %d, want 0", got)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: raceway not on the battlefield under seat 1")
	}

	e.emit(events.Event{Kind: events.ControlChange, Obj: id, Player: 0})
	if got := e.G.Players[0].Speed; got != 1 {
		t.Fatalf("seat 0 speed after gaining control: %d, want 1 (CR 702.179a SBA)", got)
	}
	// Exactly one "start your engines" grant for seat 0: the scan must not
	// re-emit on the recursive fold or on any later transfer.
	grants := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.SpeedChange && ev.Player == 0 &&
			ev.Text == "start your engines"
	})
	if grants != 1 {
		t.Fatalf("seat 0 start-your-engines grants: %d, want exactly 1", grants)
	}

	// Idempotence: transferring the carrier to seat 2 (also speed-less) starts
	// seat 2, and a transfer BACK to seat 0 -- already at speed 1 -- leaves it
	// untouched.
	e.emit(events.Event{Kind: events.ControlChange, Obj: id, Player: 2})
	if got := e.G.Players[2].Speed; got != 1 {
		t.Fatalf("seat 2 speed after gaining control: %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: id, Player: 0})
	if got := e.G.Players[0].Speed; got != 1 {
		t.Fatalf("seat 0 speed after regaining control: %d, want 1 (unchanged)", got)
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.SpeedChange && ev.Player == 0 &&
			ev.Text == "start your engines"
	}); got != 1 {
		t.Fatalf("seat 0 re-emitted the grant on a second transfer: %d, want 1", got)
	}
}

// TestStartYourEnginesSpeedStartsPastAReplacedEntry holds CR 704.5aa to having no
// entry condition: an entry an Updated replacement folds itself, with or without
// a counter on the permanent, still starts its controller's speed by the next
// state-based check. The last case folds a static-less carrier's entry raw on a
// quiet board, where the check must not skip the pass that starts the speed.
func TestStartYourEnginesSpeedStartsPastAReplacedEntry(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, carrier string
		seat0, seat1  string
		raw           bool
	}{
		{name: "counter replacement", carrier: "Streaking Oilgorger", seat0: "Thunderous Velocipede"},
		{name: "tap replacement", carrier: "Streaking Oilgorger", seat1: "Thalia, Heretic Cathar"},
		{name: "quiet entry", carrier: "Point the Way", seat0: "Grizzly Bears", raw: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck0 := []*cards.Card{lookup(t, reg, tc.carrier)}
			var deck1 []*cards.Card
			if tc.seat0 != "" {
				deck0 = append(deck0, lookup(t, reg, tc.seat0))
			}
			if tc.seat1 != "" {
				deck1 = append(deck1, lookup(t, reg, tc.seat1))
			}
			e := corpusEngine(t, reg, deck0, deck1)
			if tc.seat0 != "" {
				moveByName(t, e, 0, tc.seat0, state.ZBattlefield)
			}
			if tc.seat1 != "" {
				moveByName(t, e, 1, tc.seat1, state.ZBattlefield)
			}
			e.checkStateBased()
			if tc.raw {
				id := findSeatCard(t, e, 0, tc.carrier)
				events.Emit(e.G, e.L, events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
			} else {
				moveByName(t, e, 0, tc.carrier, state.ZBattlefield)
			}
			if got := e.G.Players[0].Speed; got != 0 && tc.raw {
				t.Fatalf("precondition: speed %d before the state-based check, want 0", got)
			}
			e.checkStateBased()
			if got := e.G.Players[0].Speed; got != 1 {
				t.Fatalf("seat 0 speed after the state-based check: %d, want 1", got)
			}
			grants := countEvents(e, func(ev events.Event) bool {
				return ev.Kind == events.SpeedChange && ev.Player == 0
			})
			if grants != 1 {
				t.Fatalf("speed grants: %d, want 1", grants)
			}
		})
	}
}

// findSeatCard is the first copy of name in seat p's hand or library.
func findSeatCard(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return id
			}
		}
	}
	t.Fatalf("card %q not found in seat %d's hand or library", name, p)
	return 0
}
