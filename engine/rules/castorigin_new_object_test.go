package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCastOriginEndsOnAnyZoneChangeAfterTheCast extends the CR 400.7 rule to
// the remaining provenance reads: WasCast (Forge's castFrom != null) and
// latestCastOrigin (Object.HasLatestCast, behind WasCastFromHand,
// WasCastFromExile and the wasCastFrom<zone> filter family). After its latest
// cast, any zone change of the object other than the resolution's stack ->
// battlefield entry makes a new object that was never cast. A countered
// creature reanimated from the graveyard must not read "cast from hand": the
// folded origin was never reset and WasCast scanned for any PutOnStack at
// all.
func TestCastOriginEndsOnAnyZoneChangeAfterTheCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Grizzly Bears"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	if !e.WasCast(id) || !e.WasCastFromHand(id) {
		t.Fatal("precondition: the Bears on the stack were cast from hand")
	}
	// Countered: the card goes to its owner's graveyard, then is reanimated.
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZGraveyard})
	if e.WasCast(id) || e.WasCastFromHand(id) {
		t.Fatal("the countered card in the graveyard is a new object that was never cast (CR 400.7)")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZBattlefield})
	if e.WasCast(id) || e.WasCastFromHand(id) {
		t.Fatal("the reanimated Bears were never cast (CR 400.7)")
	}
	if _, _, ok := e.latestCastOrigin(id); ok {
		t.Fatal("latestCastOrigin still reports a cast for the reanimated Bears")
	}

	// A cast from exile, resolved, then blinked: the cast ends at the blink.
	e3 := handEngine(t, corpusAlternativeCard(t, "Grizzly Bears"))
	id3 := e3.G.Zone(state.ZHand, 0)[0]
	e3.emit(events.Event{Kind: events.MoveZone, Obj: id3, From: state.ZHand, To: state.ZExile})
	e3.emit(events.Event{Kind: events.PutOnStack, Obj: id3, Player: 0, From: state.ZExile, To: state.ZStack})
	e3.emit(events.Event{Kind: events.MoveZone, Obj: id3, From: state.ZStack, To: state.ZBattlefield})
	if !e3.WasCastFromExile(id3) {
		t.Fatal("precondition: the resolved Bears were cast from exile")
	}
	e3.emit(events.Event{Kind: events.MoveZone, Obj: id3, From: state.ZBattlefield, To: state.ZExile})
	e3.emit(events.Event{Kind: events.MoveZone, Obj: id3, From: state.ZExile, To: state.ZBattlefield})
	if e3.WasCastFromExile(id3) || e3.WasCast(id3) {
		t.Fatal("the blinked Bears were never cast (CR 400.7)")
	}

	// Control: the resolution's own stack -> battlefield entry keeps the cast.
	e2 := handEngine(t, corpusAlternativeCard(t, "Grizzly Bears"))
	id2 := e2.G.Zone(state.ZHand, 0)[0]
	e2.emit(events.Event{Kind: events.PutOnStack, Obj: id2, Player: 0, From: state.ZHand, To: state.ZStack})
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZStack, To: state.ZBattlefield})
	if !e2.WasCast(id2) || !e2.WasCastFromHand(id2) {
		t.Fatal("a resolved permanent spell on the battlefield was cast")
	}
}

// TestThisTurnCastLedgerReadsTheCastNotTheObject: the this-turn cast ledger
// counts a cast as it was made, so an instant cast from hand that resolved
// into the graveyard still counts for "a spell cast from your hand this
// turn" (Forge's ThisTurnCast list holds cast-time copies). The object in
// the graveyard is a new object that was never cast (CR 400.7), so the
// ledger must read the logged cast, not the object's current provenance.
func TestThisTurnCastLedgerReadsTheCastNotTheObject(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Delayed Blast Fireball"))
	id := e.G.Obj(e.G.Zone(state.ZHand, 0)[0]).ID
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 1, 2
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: the Fireball resolved into the graveyard, zone %v", o.Zone)
	}
	if e.WasCastFromHandByYou(id, 0) || e.WasCast(id) {
		t.Fatal("the resolved card in the graveyard is a new object that was never cast (CR 400.7)")
	}
	noTC := effects.TriggerContext{}
	for _, spec := range []string{"Card.wasCastFromYourHandByYou", "Card.wasCastByYou", "Card.wasCastFromYourHand", "Card.wasCast"} {
		if got := e.SpellsCastThisTurnMatching(0, spec, noTC); got != 1 {
			t.Errorf("SpellsCastThisTurnMatching(%q) = %d after the spell resolved, want 1", spec, got)
		}
	}
}
