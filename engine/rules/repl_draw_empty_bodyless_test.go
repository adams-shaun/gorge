// The bodyless arm of CR 614.11: a Draw replacement with Prevent$ and no
// With body (Living Conundrum's "if you would draw a card while your library
// has no cards in it, skip that draw instead") meets an attempted draw from
// an empty library exactly like a bodied one (Laboratory Maniac's win),
// so the draw is skipped and the CR 704.5b loss never follows.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLivingConundrumSkipsEmptyLibraryDraw pins the skip: with Living
// Conundrum on the battlefield and an empty library, a draw is prevented and
// the player neither loses nor is marked; with the Conundrum gone the same
// draw loses, so the pass cannot come from the draw path being inert.
func TestLivingConundrumSkipsEmptyLibraryDraw(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) (*Engine, Config, state.ObjID) {
		t.Helper()
		reg := testutil.CorpusRegistry(t)
		lc := mustCorpusCard(t, reg, "Living Conundrum")
		if d := lc.Link(); len(d) != 0 {
			t.Fatalf("link Living Conundrum: %v", d)
		}
		seatDeck := append([]*cards.Card{lc}, mountainDeck(t, 39)...)
		cfg := seatZeroStart(Config{Seed: 8340, Names: []string{"a", "b"},
			Decks:  [][]*cards.Card{seatDeck, mountainDeck(t, 40)},
			Tokens: reg.Tokens, NameUniverse: reg.Universe()})
		e := New(cfg)
		e.Advance()
		id := moveNamedToBattlefield(t, e, 0, "Living Conundrum")
		return e, cfg, id
	}

	e, cfg, id := build(t)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Living Conundrum is not on seat 0's battlefield: %+v", o)
	}
	hasBodylessDraw := false
	for i := range o.Face().Repls {
		if o.Face().Repls[i].Event == "Draw" && o.Face().Repls[i].With == nil {
			hasBodylessDraw = true
		}
	}
	if !hasBodylessDraw {
		t.Fatal("precondition: Living Conundrum carries no parsed bodyless Event$ Draw line")
	}
	cantLoseEmptyLibrary(t, e, 0)
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("precondition: seat 0 library holds %d cards, want 0", n)
	}

	e.drawCard(0)
	if e.G.Players[0].DrewFromEmpty || countDrawEmpty(e, 0, 0) != 0 {
		t.Fatal("the empty-library draw was marked instead of skipped")
	}
	e.checkStateBased()
	if e.G.Players[0].Lost || e.G.Over {
		t.Fatalf("seat 0 lost=%v over=%v although Living Conundrum skips the draw",
			e.G.Players[0].Lost, e.G.Over)
	}
	replayCheck(t, e, cfg)

	// Control: the same draw with the Conundrum gone loses (CR 704.5b), so the
	// first half cannot pass because the draw path never marks.
	e2, cfg2, id2 := build(t)
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZBattlefield, To: state.ZGraveyard})
	cantLoseEmptyLibrary(t, e2, 0)
	e2.drawCard(0)
	e2.checkStateBased()
	if !e2.G.Players[0].Lost {
		t.Fatal("control: seat 0 must lose to the empty library once Living Conundrum is gone")
	}
	replayCheck(t, e2, cfg2)
}
