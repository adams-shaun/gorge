package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR704_5iLoyaltyXWalkerAtZeroDies: Nissa, Steward of Elements has printed
// loyalty X. CR 107.3/208.x makes X 0 anywhere but the stack, so a Nissa put
// onto the battlefield with no loyalty counters has loyalty 0 and CR 704.5i
// puts her into her owner's graveyard.
func TestCR704_5iLoyaltyXWalkerAtZeroDies(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Nissa, Steward of Elements")
	id := ids["Nissa, Steward of Elements"]
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.FaceDown {
		t.Fatalf("precondition: zone %v face-down %v, want a face-up battlefield permanent", o.Zone, o.FaceDown)
	}
	if f := o.Face(); f == nil || !f.IsPlaneswalker() || f.Loyalty != "X" {
		t.Fatalf("precondition: face %+v, want a printed planeswalker with Loyalty:X", f)
	}
	if got := o.Counter("LOYALTY"); got != 0 {
		t.Fatalf("precondition: loyalty counters %d, want 0", got)
	}
	e.checkStateBased()
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("CR 704.5i: Nissa with loyalty X (0 outside the stack) and no loyalty counters is in %v, want the graveyard", z)
	}
}

// TestCR306_5bLoyaltyXWalkerCastForXEntersWithXCounters: the other half of the
// loyalty-X rule. Cast for X = 2, Nissa enters with two loyalty counters
// (CR 107.3/306.5b), so the CR 704.5i sweep leaves her on the battlefield.
func TestCR306_5bLoyaltyXWalkerCastForXEntersWithXCounters(t *testing.T) {
	t.Parallel()
	const x = 2
	e := handEngine(t, card(t, corpusCardText(t, "n/nissa_steward_of_elements.txt")))
	id := e.G.Zone(state.ZHand, 0)[0]
	if f := e.G.Obj(id).Face(); f == nil || f.Loyalty != "X" {
		t.Fatalf("precondition: face %+v, want printed Loyalty:X", f)
	}
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 3})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	e.askPriority(0)
	cr601Choose(t, e, "cast Nissa", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == id })
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("want the X announcement, got %+v", d)
	}
	cr601Choose(t, e, "announce X", func(o decision.Option) bool { return o.Kind == "x" && o.Amount == x })
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Nissa cast for X=%d is in %v, want the battlefield", x, o.Zone)
	}
	if got := o.Counter("LOYALTY"); got != x {
		t.Fatalf("loyalty counters = %d, want %d", got, x)
	}
}
