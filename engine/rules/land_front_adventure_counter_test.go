package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCounteredAdventureExiledIsNotPlayable pins the resolution boundary of
// CR 715.3d: the card is exiled and playable "as it resolves", so an
// Adventure spell countered to exile never earns the permission. Two counter
// shapes reach exile: Dissipate's direct Destination$ Exile, and an ordinary
// counter whose graveyard move Rest in Peace redirects to exile. The same
// positive provenance marker gates both the land play (the land-front
// Adventure) and the non-land recast, so both are covered here. The
// precondition each subtest asserts is that the Adventure really did not
// resolve (the mill target's library and the bounce target are untouched)
// while the counter really did put the card in exile.
func TestCounteredAdventureExiledIsNotPlayable(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	island := searchCorpusCard(t, reg, "Island")
	dissipate := searchCorpusCard(t, reg, "Dissipate")

	t.Run("land front", func(t *testing.T) {
		t.Parallel()
		jidoor := searchCorpusCard(t, reg, "Jidoor, Aristocratic Capital")
		deck := []*cards.Card{jidoor}
		for len(deck) < 40 {
			deck = append(deck, island)
		}
		opp := []*cards.Card{dissipate}
		for len(opp) < 40 {
			opp = append(opp, island)
		}
		cfg := seatZeroStart(Config{Seed: 7405, Names: []string{"adventure", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		id := searchMoveByName(t, e, "Jidoor, Aristocratic Capital", state.ZHand)
		diss := searchMoveByNameSeat(t, e, 1, "Dissipate", state.ZHand)
		addMana(t, e, 0, "UUUUUU")
		addMana(t, e, 1, "CUU")

		libraryBefore := len(e.G.Zone(state.ZLibrary, 1))
		counterAdventureToExile(t, e, id, diss, 0, 1)
		if got := len(e.G.Zone(state.ZLibrary, 1)); got != libraryBefore {
			t.Fatalf("precondition: Overture resolved (opponent library %d -> %d), want the counter to stop it",
				libraryBefore, got)
		}
		if got := e.G.Obj(diss).Zone; got != state.ZGraveyard {
			t.Fatalf("precondition: Dissipate zone = %s, want graveyard", got)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZExile || int(o.FaceIdx) != 0 {
			t.Fatalf("precondition: countered Jidoor zone=%s faceIdx=%d, want ZExile/0", o.Zone, o.FaceIdx)
		}

		driveToStep(t, e, 1, 0, state.StepMain2)
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("expected seat 0's priority in main2, got %+v", d)
		}
		if e.G.Players[0].LandsPlayed != 0 {
			t.Fatalf("precondition: LandsPlayed = %d, want 0 so a play would be offered if legal", e.G.Players[0].LandsPlayed)
		}
		for _, opt := range d.Options {
			if opt.Obj == id && (opt.Kind == "play_land" || opt.Mode == "adventure_recast") {
				t.Fatalf("countered Adventure offered %q from exile: %+v", opt.Kind, opt)
			}
		}
		replayCheck(t, e, cfg)
	})

	t.Run("non-land front", func(t *testing.T) {
		t.Parallel()
		borrower := searchCorpusCard(t, reg, "Brazen Borrower")
		bear := searchCorpusCard(t, reg, "Grizzly Bears")
		deck := []*cards.Card{borrower}
		for len(deck) < 40 {
			deck = append(deck, island)
		}
		opp := []*cards.Card{dissipate, bear}
		for len(opp) < 40 {
			opp = append(opp, island)
		}
		cfg := seatZeroStart(Config{Seed: 7406, Names: []string{"adventure", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		id := searchMoveByName(t, e, "Brazen Borrower", state.ZHand)
		diss := searchMoveByNameSeat(t, e, 1, "Dissipate", state.ZHand)
		bearID := searchMoveByNameSeat(t, e, 1, "Grizzly Bears", state.ZBattlefield)
		addMana(t, e, 0, "CUU")
		addMana(t, e, 1, "CUU")

		counterAdventureToExile(t, e, id, diss, bearID, 0)
		if got := e.G.Obj(bearID).Zone; got != state.ZBattlefield {
			t.Fatalf("precondition: Petty Theft resolved (bear zone %s), want the counter to stop it", got)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZExile || int(o.FaceIdx) != 0 {
			t.Fatalf("precondition: countered Borrower zone=%s faceIdx=%d, want ZExile/0", o.Zone, o.FaceIdx)
		}

		driveToStep(t, e, 1, 0, state.StepMain2)
		// Fund the recast's {1}{U}{U} in main2 itself: mana added in main1
		// empties on the step change, and addMana would drive back to main1.
		for _, r := range "CUU" {
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
		}
		e.priorityRound()
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("expected seat 0's priority in main2, got %+v", d)
		}
		if got := adventureOption(t, e, id, "adventure_recast"); got != nil {
			t.Fatalf("countered Adventure offered as adventure_recast: %+v", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("land front redirected to exile", func(t *testing.T) {
		t.Parallel()
		jidoor := searchCorpusCard(t, reg, "Jidoor, Aristocratic Capital")
		rip := searchCorpusCard(t, reg, "Rest in Peace")
		counterspell := searchCorpusCard(t, reg, "Counterspell")
		deck := []*cards.Card{rip, jidoor}
		for len(deck) < 40 {
			deck = append(deck, island)
		}
		opp := []*cards.Card{counterspell}
		for len(opp) < 40 {
			opp = append(opp, island)
		}
		cfg := seatZeroStart(Config{Seed: 7407, Names: []string{"adventure", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		putRestInPeace(t, e)
		id := searchMoveByName(t, e, "Jidoor, Aristocratic Capital", state.ZHand)
		counterID := searchMoveByNameSeat(t, e, 1, "Counterspell", state.ZHand)
		addMana(t, e, 0, "UUUUUU")
		addMana(t, e, 1, "UU")

		libraryBefore := len(e.G.Zone(state.ZLibrary, 1))
		counterAdventureToExile(t, e, id, counterID, 0, 1)
		if got := len(e.G.Zone(state.ZLibrary, 1)); got != libraryBefore {
			t.Fatalf("precondition: Overture resolved (opponent library %d -> %d), want the counter to stop it",
				libraryBefore, got)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZExile || int(o.FaceIdx) != 0 {
			t.Fatalf("precondition: redirected Jidoor zone=%s faceIdx=%d, want ZExile/0", o.Zone, o.FaceIdx)
		}
		assertNoAdventureZoneMarker(t, e, id)

		driveToStep(t, e, 1, 0, state.StepMain2)
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("expected seat 0's priority in main2, got %+v", d)
		}
		if e.G.Players[0].LandsPlayed != 0 {
			t.Fatalf("precondition: LandsPlayed = %d, want 0 so a play would be offered if legal", e.G.Players[0].LandsPlayed)
		}
		for _, opt := range d.Options {
			if opt.Obj == id && (opt.Kind == "play_land" || opt.Mode == "adventure_recast") {
				t.Fatalf("redirected countered Adventure offered %q from exile: %+v", opt.Kind, opt)
			}
		}
		replayCheck(t, e, cfg)
	})

	t.Run("non-land front redirected to exile", func(t *testing.T) {
		t.Parallel()
		borrower := searchCorpusCard(t, reg, "Brazen Borrower")
		bear := searchCorpusCard(t, reg, "Grizzly Bears")
		rip := searchCorpusCard(t, reg, "Rest in Peace")
		counterspell := searchCorpusCard(t, reg, "Counterspell")
		deck := []*cards.Card{rip, borrower}
		for len(deck) < 40 {
			deck = append(deck, island)
		}
		opp := []*cards.Card{counterspell, bear}
		for len(opp) < 40 {
			opp = append(opp, island)
		}
		cfg := seatZeroStart(Config{Seed: 7408, Names: []string{"adventure", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		putRestInPeace(t, e)
		id := searchMoveByName(t, e, "Brazen Borrower", state.ZHand)
		counterID := searchMoveByNameSeat(t, e, 1, "Counterspell", state.ZHand)
		bearID := searchMoveByNameSeat(t, e, 1, "Grizzly Bears", state.ZBattlefield)
		addMana(t, e, 0, "CUU")
		addMana(t, e, 1, "UU")

		counterAdventureToExile(t, e, id, counterID, bearID, 0)
		if got := e.G.Obj(bearID).Zone; got != state.ZBattlefield {
			t.Fatalf("precondition: Petty Theft resolved (bear zone %s), want the counter to stop it", got)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZExile || int(o.FaceIdx) != 0 {
			t.Fatalf("precondition: redirected Borrower zone=%s faceIdx=%d, want ZExile/0", o.Zone, o.FaceIdx)
		}
		assertNoAdventureZoneMarker(t, e, id)

		driveToStep(t, e, 1, 0, state.StepMain2)
		// Fund the recast's {1}{U}{U} in main2 itself: mana added in main1
		// empties on the step change, and addMana would drive back to main1.
		for _, r := range "CUU" {
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
		}
		e.priorityRound()
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
			t.Fatalf("expected seat 0's priority in main2, got %+v", d)
		}
		if got := adventureOption(t, e, id, "adventure_recast"); got != nil {
			t.Fatalf("redirected countered Adventure offered as adventure_recast: %+v", got)
		}
		replayCheck(t, e, cfg)
	})
}

// putRestInPeace moves a real Rest in Peace from seat 0's deck onto the
// battlefield through a logged move (so replayCheck sees it) and drains its
// "exile all graveyards" ETB trigger. Its graveyard replacement is global, so
// the controller does not matter to the redirect under test.
func putRestInPeace(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	id := searchMoveByName(t, e, "Rest in Peace", state.ZBattlefield)
	e.putTriggersOnStack()
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Rest in Peace zone = %+v, want battlefield", o)
	}
	return id
}

// assertNoAdventureZoneMarker pins the shape the positive provenance check
// depends on: the redirect's own exile move carries no adventure-zone marker,
// so only the resolution marker (not the absence of a counter label) can tell
// a resolved Adventure from a countered one.
func assertNoAdventureZoneMarker(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZExile {
			if ev.Counter == adventureZoneMoveMarker {
				t.Fatalf("precondition: redirect's exile move carries the adventure marker: %+v", ev)
			}
			return
		}
	}
	t.Fatalf("precondition: no exile move for %d", id)
}

// counterAdventureToExile casts the Adventure face of spellID, answers its
// target ask (targetObj when non-zero, else the player targetPlayer), passes
// priority to seat 1, casts counterID targeting the Adventure spell, and
// drains the stack. Both direct-exile (Dissipate) and graveyard-redirect
// (Counterspell under Rest in Peace) callers use it; each asserts the counter
// prevented resolution and that the card reached exile.
func counterAdventureToExile(t *testing.T, e *Engine, spellID, dissID, targetObj state.ObjID, targetPlayer state.PlayerID) {
	t.Helper()
	alt := adventureOption(t, e, spellID, "adventure_alt")
	if alt == nil {
		t.Fatalf("Adventure face not offered from hand: %+v", castOptions(t, e))
	}
	submitChoices(t, e, alt.Index)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		idx := -1
		for _, o := range d.Options {
			if targetObj != 0 && o.Obj == targetObj {
				idx = o.Index
			}
			if targetObj == 0 && o.Kind == "player" && o.Player == targetPlayer {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no Adventure target option (obj %d, player %d): %+v", targetObj, targetPlayer, d.Options)
		}
		submitChoices(t, e, idx)
	}
	passToSeat(t, e, 1)
	submitChoices(t, e, castOptionFor(t, e, dissID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("counter spell target ask: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == spellID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("counter has no option targeting the Adventure spell: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 30)
}
