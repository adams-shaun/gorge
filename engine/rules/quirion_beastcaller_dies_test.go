package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// quirionBoard builds a two-seat game with Quirion Beastcaller and two
// Grizzly Bears on seat 0's battlefield, at seat 0's first main phase. The
// counters are placed (or not) by the caller.
func quirionBoard(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Quirion Beastcaller"), bears, bears}, mountainDeck(t, 37)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	q := findAndMoveToHand(t, e, 0, "Quirion Beastcaller")
	moveToBattlefield(t, e, q)
	b1 := findAndMoveToHand(t, e, 0, "Grizzly Bears")
	moveToBattlefield(t, e, b1)
	b2 := findAndMoveToHand(t, e, 0, "Grizzly Bears")
	moveToBattlefield(t, e, b2)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	return e, q, b1, b2
}

// TestQuirionBeastcallerDiesDistributesCounters pins the CR 603.10a/608.2h
// last-known-information sizing of a dies trigger whose TargetMax$ and
// DividedAsYouChoose$ both read TriggeredCard$CardCounters: the placement ask
// must see the counters the creature had as it left, not the graveyard card's
// zero, so three counters offer up to three targets and a real division.
func TestQuirionBeastcallerDiesDistributesCounters(t *testing.T) {
	t.Run("three counters", func(t *testing.T) {
		e, q, b1, b2 := quirionBoard(t)
		e.emit(events.Event{Kind: events.CounterChange, Obj: q, Counter: "P1P1", Amount: 3})
		if got := e.G.Obj(q).Counter("P1P1"); got != 3 {
			t.Fatalf("precondition: Quirion Beastcaller counters = %d, want 3", got)
		}
		e.emit(events.Event{Kind: events.Damage, Obj: q, Amount: 9})
		e.checkStateBased()
		e.priorityRound()

		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("after the lethal damage: %+v, want the dies trigger's target ask", d)
		}
		if d.Min != 0 || d.Max != 3 {
			t.Fatalf("target bounds = %d..%d, want 0..3 (TargetMax$ X from the 3 last-known counters)", d.Min, d.Max)
		}
		for _, o := range d.Options {
			if o.Obj != b1 && o.Obj != b2 {
				t.Fatalf("target option %d is not one of the two Bears: %+v", o.Obj, o)
			}
		}
		submitChoices(t, e, optForObj(t, d, b1), optForObj(t, d, b2))

		dd := e.Pending()
		if !isCastDivision(dd) {
			t.Fatalf("after the targets: %+v, want the CR 601.2d division ask", dd)
		}
		if dd.Min != 1 || dd.Max != 1 {
			t.Fatalf("division picks = %d..%d, want 1..1 (3 counters over 2 targets)", dd.Min, dd.Max)
		}
		submitChoices(t, e, optForObj(t, dd, b1))

		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(b1).Counter("P1P1"); got != 2 {
			t.Fatalf("first Bear counters = %d, want 2 (the division's extra point)", got)
		}
		if got := e.G.Obj(b2).Counter("P1P1"); got != 1 {
			t.Fatalf("second Bear counters = %d, want 1", got)
		}
	})

	t.Run("zero counters", func(t *testing.T) {
		e, q, b1, b2 := quirionBoard(t)
		if got := e.G.Obj(q).Counter("P1P1"); got != 0 {
			t.Fatalf("precondition: Quirion Beastcaller counters = %d, want 0", got)
		}
		e.emit(events.Event{Kind: events.Damage, Obj: q, Amount: 9})
		e.checkStateBased()
		e.priorityRound()

		if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
			t.Fatalf("zero-counter dies trigger posed %+v, want no target ask", d)
		}
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(b1).Counter("P1P1"); got != 0 {
			t.Fatalf("first Bear counters = %d, want 0", got)
		}
		if got := e.G.Obj(b2).Counter("P1P1"); got != 0 {
			t.Fatalf("second Bear counters = %d, want 0", got)
		}
	})
}
