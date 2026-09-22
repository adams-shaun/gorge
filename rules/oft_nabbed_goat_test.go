package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Oft-Nabbed Goat's death trigger (task defined-triggered-card-owner) pinned
// end to end on the real corpus card. Its compiled body is
//
//	SVar:TrigDeath:DB$ Draw | Defined$ TriggeredCardOwner | NumCards$ X | SubAbility$ TrigDrain
//	SVar:TrigDrain:DB$ LoseLife | Defined$ NonTriggeredCardOwner | LifeAmount$ X
//	SVar:X:TriggeredCard$CardCounters.M1M1
//
// so before the fix Defined$ fell through to the resolving source for the
// draw leg (an accidental draw) and to the source for the drain leg -- the
// drain direction was the player-visible defect: the OWNER lost life instead
// of every OTHER player. This test puts two -1/-1 counters on the Goat, kills
// it, and asserts the owner (seat 2, distinct from the Goat's controller seat
// 0) draws exactly two while the two non-owner seats each lose exactly two and
// the owner loses none.

// goatDies seats Oft-Nabbed Goat under seat 0's CONTROL while its OWNER is
// seat 2, puts two M1M1 counters on it and kills it with lethal damage,
// draining the death trigger to completion. Three seats, all corpus cards.
func goatDies(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	goat, ok := reg.Lookup("Oft-Nabbed Goat")
	if !ok {
		t.Fatalf("Oft-Nabbed Goat missing from corpus")
	}
	if d := goat.Link(); len(d) != 0 {
		t.Fatalf("link Oft-Nabbed Goat: %v", d)
	}
	names := []string{"a", "b", "c"}
	decks := make([][]*cards.Card, 3)
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	decks[0] = append(decks[0], goat)
	cfg := seatZeroStart(Config{Seed: 197, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()

	var gid state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Oft-Nabbed Goat" {
			gid = id
		}
	}
	if gid == 0 {
		t.Fatalf("Oft-Nabbed Goat was not dealt to seat 0")
	}
	// The Goat must be OWNED by a seat other than its controller's, which is
	// the whole point of the test -- otherwise the owner/controller readings
	// coincide and the assertions below prove nothing. It is dealt to seat 0
	// (owner 0) and then stolen by seat 1 via a real ControlChange, so owner 0
	// != controller 1.
	e.emit(events.Event{Kind: events.MoveZone, Obj: gid, From: state.ZLibrary, To: state.ZBattlefield})
	e.priorityRound()
	e.emit(events.Event{Kind: events.ControlChange, Obj: gid, Player: 1})
	if o := e.G.Obj(gid); o.Owner != 0 || o.Controller != 1 {
		t.Fatalf("fixture precondition: Goat owner=%d controller=%d, want owner 0 != controller 1", o.Owner, o.Controller)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: gid, Counter: "M1M1", Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Obj: gid, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	return e, cfg, gid
}

// handSize returns seat p's hand count.
func handSize(e *Engine, p state.PlayerID) int { return len(e.G.Zone(state.ZHand, p)) }

func TestOftNabbedGoatOwnerDrawsAndOtherPlayersLose(t *testing.T) {
	e, cfg, gid := goatDies(t)

	// Precondition: the two counters really were on it and it really died.
	// The trigger's SVar:X reads TriggeredCard$CardCounters.M1M1, so a
	// fixture that left the counters off would make X zero and the whole
	// test vacuous.
	if o := e.G.Obj(gid); o != nil {
		if o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: Goat zone = %v, want graveyard (it must have died)", o.Zone)
		}
	}

	// Owner seat 0 drew exactly two; the two other seats drew none.
	if got := handSize(e, 0); got != 2+7 {
		t.Fatalf("owner (seat 0) hand = %d, want 7 opening + 2 trigger draws = 9", got)
	}
	for _, p := range []state.PlayerID{1, 2} {
		if got := handSize(e, p); got != 7 {
			t.Fatalf("non-owner seat %d hand = %d, want the untouched opening 7", p, got)
		}
	}

	// The owner loses no life; each other living player loses exactly two.
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("owner (seat 0) life = %d, want 20 (it must not drain itself)", got)
	}
	for _, p := range []state.PlayerID{1, 2} {
		if got := e.G.Players[p].Life; got != 18 {
			t.Fatalf("non-owner seat %d life = %d, want 18 (it must lose exactly 2)", p, got)
		}
	}

	replayCheck(t, e, cfg)
}
