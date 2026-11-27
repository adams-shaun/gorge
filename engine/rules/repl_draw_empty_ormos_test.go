// The counter arm of CR 614.11: a mandatory Draw replacement whose body is a
// PutCounter on Self (Ormos, Archive Keeper's "if you would draw a card while
// your library has no cards in it, instead put five +1/+1 counters on
// CARDNAME") meets an attempted draw from an empty library: the counters land,
// no DrawEmpty mark follows, and the CR 704.5b loss never fires.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOrmosArchiveKeeperCountersOnEmptyLibraryDraw pins the counter arm: with
// Ormos, Archive Keeper on the battlefield and an empty library, an attempted
// draw puts five +1/+1 counters on it, records no DrawEmpty mark, and the
// player neither is marked nor loses; with Ormos gone the same draw loses, so
// the pass cannot come from the draw path being inert.
func TestOrmosArchiveKeeperCountersOnEmptyLibraryDraw(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) (*Engine, Config, state.ObjID) {
		t.Helper()
		reg := testutil.CorpusRegistry(t)
		ormos := mustCorpusCard(t, reg, "Ormos, Archive Keeper")
		if d := ormos.Link(); len(d) != 0 {
			t.Fatalf("link Ormos, Archive Keeper: %v", d)
		}
		seatDeck := append([]*cards.Card{ormos}, mountainDeck(t, 39)...)
		cfg := seatZeroStart(Config{Seed: 8341, Names: []string{"a", "b"},
			Decks:  [][]*cards.Card{seatDeck, mountainDeck(t, 40)},
			Tokens: reg.Tokens, NameUniverse: reg.Universe()})
		e := New(cfg)
		e.Advance()
		id := moveNamedToBattlefield(t, e, 0, "Ormos, Archive Keeper")
		return e, cfg, id
	}

	e, cfg, id := build(t)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Ormos is not on seat 0's battlefield: %+v", o)
	}
	hasCounterRepl := false
	for i := range o.Face().Repls {
		r := &o.Face().Repls[i]
		if r.Event == "Draw" && r.With != nil && r.With.API != "Draw" {
			hasCounterRepl = true
		}
	}
	if !hasCounterRepl {
		t.Fatal("precondition: Ormos carries no parsed non-draw-bodied Event$ Draw line")
	}
	cantLoseEmptyLibrary(t, e, 0)
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("precondition: seat 0 library holds %d cards, want 0", n)
	}

	e.drawCard(0)
	if e.G.Players[0].DrewFromEmpty || countDrawEmpty(e, 0, 0) != 0 {
		t.Fatal("the empty-library draw was marked instead of replaced")
	}
	o = e.G.Obj(id)
	if o == nil {
		t.Fatal("Ormos left the battlefield during the replacement")
	}
	counters := 0
	for i := range o.Counters {
		if o.Counters[i].Kind == "P1P1" {
			counters += int(o.Counters[i].N)
		}
	}
	if counters != 5 {
		t.Fatalf("Ormos carries %d +1/+1 counters after the replaced draw, want 5", counters)
	}
	e.checkStateBased()
	if e.G.Players[0].Lost || e.G.Over {
		t.Fatalf("seat 0 lost=%v over=%v although Ormos replaces the draw",
			e.G.Players[0].Lost, e.G.Over)
	}
	replayCheck(t, e, cfg)

	// Control: the same draw with Ormos gone loses (CR 704.5b), so the first
	// half cannot pass because the draw path never marks.
	e2, cfg2, id2 := build(t)
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZBattlefield, To: state.ZGraveyard})
	cantLoseEmptyLibrary(t, e2, 0)
	e2.drawCard(0)
	e2.checkStateBased()
	if !e2.G.Players[0].Lost {
		t.Fatal("control: seat 0 must lose to the empty library once Ormos is gone")
	}
	replayCheck(t, e2, cfg2)
}
