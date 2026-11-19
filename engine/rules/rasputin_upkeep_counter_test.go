// Rasputin Dreamweaver's upkeep trigger reads the
// `Card.Self+startedTheTurnUntapped` filter property (Forge's
// Card.isStartedTheTurnUntapped, CR 502.3's untap step): the permanent was
// untapped as its controller's turn began, so a permanent tapped during the
// previous turn fails the intervening-if at the next upkeep even though that
// player's untap step untapped it. The property needs a per-object stamp the
// TurnChange fold writes, so the test exercises the real engine path —
// trigger, intervening-if and counter fold — on the corpus card.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// rasputinDreamers puts the corpus Rasputin on seat 0's battlefield during
// turn 1's main phase, drops its dream counters to six (it enters with the
// MaxCounter cap of seven, so the cap would absorb the trigger's counter and
// a passing test could not tell added from capped), and returns the object id.
func rasputinDreamers(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg := edhbGame(t, seed, []string{"Rasputin Dreamweaver"}, nil, nil, nil)
	id := moveToBattlefieldByName(t, e, 0, "Rasputin Dreamweaver")
	e.pending = nil
	e.Advance()
	if got := e.G.Obj(id).Counter("DREAM"); got != 7 {
		t.Fatalf("precondition: Rasputin entered with %d dream counters, want 7", got)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "DREAM", Amount: -1})
	e.pending = nil
	e.Advance()
	if got := e.G.Obj(id).Counter("DREAM"); got != 6 {
		t.Fatalf("precondition: Rasputin has %d dream counters after the drop, want 6", got)
	}
	return e, cfg, id
}

// driveToUpkeep passes priority through the following turns to seat 0's turn 3
// upkeep and resolves whatever the step put on the stack.
func driveToUpkeep(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	var rasputin state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Rasputin Dreamweaver" {
			rasputin = id
		}
	}
	if rasputin == 0 {
		t.Fatal("precondition: Rasputin not on seat 0's battlefield")
	}
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	edhbDrain(t, e)
	return rasputin
}

func TestRasputinUpkeepCounterWhenStartedUntapped(t *testing.T) {
	t.Run("untapped at turn start gains a dream counter", func(t *testing.T) {
		e, cfg, id := rasputinDreamers(t, 931)
		driveToUpkeep(t, e)
		if got := e.G.Obj(id).Counter("DREAM"); got != 7 {
			t.Fatalf("Rasputin has %d dream counters at turn 3's upkeep, want 7 (the trigger put one)", got)
		}
		// The predicate must be census-recognised too, so a new never-match
		// word cannot hide behind a silent UnknownPredicates miss again.
		if un := effects.UnknownPredicates("Card.Self+startedTheTurnUntapped"); len(un) != 0 {
			t.Errorf("UnknownPredicates(Card.Self+startedTheTurnUntapped) = %v, want empty", un)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("tapped during the previous turn does not", func(t *testing.T) {
		e, cfg, id := rasputinDreamers(t, 931)
		e.emit(events.Event{Kind: events.Tap, Obj: id, Player: 0})
		e.pending = nil
		if !e.G.Obj(id).Tapped {
			t.Fatal("precondition: tap fold did not tap Rasputin")
		}
		e.Advance()
		driveToUpkeep(t, e)
		// The turn 3 untap step untapped it again, so a filter that read the
		// CURRENT status would pass here; only a stamp taken at the turn's
		// beginning keeps the trigger off.
		if e.G.Obj(id).Tapped {
			t.Fatal("precondition: the untap step did not untap Rasputin")
		}
		if got := e.G.Obj(id).Counter("DREAM"); got != 6 {
			t.Fatalf("Rasputin has %d dream counters at turn 3's upkeep, want 6 (no trigger)", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("exerted the previous turn stays tapped and does not", func(t *testing.T) {
		e, cfg, id := rasputinDreamers(t, 931)
		e.emit(events.Event{Kind: events.Tap, Obj: id, Player: 0})
		e.emit(events.Event{Kind: events.Exert, Obj: id, Player: 0})
		e.pending = nil
		e.Advance()
		driveToUpkeep(t, e)
		if !e.G.Obj(id).Tapped {
			t.Fatal("precondition: Rasputin untapped despite its exerted skip")
		}
		if got := e.G.Obj(id).Counter("DREAM"); got != 6 {
			t.Fatalf("Rasputin has %d dream counters at turn 3's upkeep, want 6 (no trigger)", got)
		}
		replayCheck(t, e, cfg)
	})
}
