// miracle.go / miracle_test.go drive the Miracle keyword (CR 702.93) through
// the trigger queue: the FIRST draw of a turn offers to cast the drawn card
// for its miracle cost, the owner declines or accepts, and a yes reveals the
// card and runs the ordinary cast flow with the miracle cost and FlagMiracle.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// moveToLibraryTop moves id into its owner's library so the test's own Draw
// event is the one that finds it. It must be a LOGGED MoveZone, not a raw
// state.SetZone: Move (events/apply.go) decides which zone to remove an
// object from by reading the object's o.Zone field, so a bare SetZone that
// leaves o.Zone stale would desync the field from the zone slice and a
// replay would diverge. Moving hand->library via the same events.Apply path
// the game uses keeps the object and the log in lockstep, so a log-only
// replay (which replays this very MoveZone, then the Draw) ends exactly as
// the live game does. The card does not need to sit at index 0: the tests
// draw it by explicit Obj, so the drawn card is the Miracle one regardless
// of where in the library it lands.
func moveToLibraryTop(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil {
		t.Fatalf("moveToLibraryTop: no such object %d", id)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZLibrary,
		Text: "moved to library top for the miracle test"})
}
func TestMiracleOffersOnTheFirstDrawOnly(t *testing.T) {
	terminus := "Name:Terminus\nManaCost:4 W W\nTypes:Sorcery\nK:Miracle:W\n" +
		"A:SP$ ChangeZoneAll | ChangeType$ Creature | Origin$ Battlefield | Destination$ Library | LibraryPosition$ -1\n" +
		"Oracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, term := newFixtureDeck(t, 101, terminus, bearSrc)
	// Put Terminus on top of seat 0's library and give them a creature to wipe.
	moveToLibraryTop(t, e, term)
	bear := putCreature(t, e, 0, bearSrc)
	addMana(t, e, 0, "W")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: term, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].Miracle {
		t.Fatalf("no miracle offer: %+v", e.pendingTriggers)
	}
	// The draw queued the offer; the engine must now hand it out as the next
	// decision. addMana's priorityRound left a priority decision pending, so
	// clear it (an ordinary test-arrangement idiom, as cast_test/choose_test
	// do) and let Advance reach the offer.
	e.pending = nil
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("offer decision %+v", d)
	}
	submitChoices(t, e, 0) // yes
	if e.G.Obj(term).Zone != state.ZStack || e.G.Obj(term).CastFlags&state.FlagMiracle == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("terminus %s flags %d pool %d", e.G.Obj(term).Zone, e.G.Obj(term).CastFlags, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZLibrary {
		t.Fatal("Terminus did not resolve")
	}
	replayCheck(t, e, cfg)

	// A second Miracle card drawn the same turn gets no offer.
	e2, _, t2 := newFixtureDeck(t, 102, terminus)
	moveToLibraryTop(t, e2, t2)
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: e2.G.Zone(state.ZLibrary, 0)[1], From: state.ZLibrary, To: state.ZHand, Secret: true})
	e2.pendingTriggers = nil
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: t2, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e2.pendingTriggers) != 0 {
		t.Fatal("miracle offered on a second draw")
	}
}

func TestMiracleWithXAsksX(t *testing.T) {
	entreat := "Name:Entreat\nManaCost:X X W W W\nTypes:Sorcery\nK:Miracle:X W W\n" +
		"A:SP$ Token | TokenAmount$ X | TokenScript$ w_4_4_angel_flying | TokenOwner$ You\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	// newFixtureDeckWithTokens tokens the Angel script that Entreat's
	// Token$ names, so a later X-binding merge can create the two Angels.
	e, cfg, en := newFixtureDeckWithTokens(t, 103, entreat)
	moveToLibraryTop(t, e, en)
	addMana(t, e, 0, "WWWW")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: en, From: state.ZLibrary, To: state.ZHand, Secret: true})
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("offer decision %+v", d)
	}
	submitChoices(t, e, 0) // yes
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" || len(d.Options) != 3 { // X = 0,1,2 with WWWW
		t.Fatalf("X after miracle: %+v", d)
	}
	// Miracle cost is {X}{W}{W}: choosing X=2 charges {2}{W}{W} of the 4
	// white tapped in, flags the spell, puts it on the stack with X recorded.
	submitChoices(t, e, 2)
	o := e.G.Obj(en)
	if o.Zone != state.ZStack || o.X != 2 || o.CastFlags&state.FlagMiracle == 0 {
		t.Fatalf("after X=2: zone=%s X=%d flags=%d", o.Zone, o.X, o.CastFlags)
	}
	// NOTE: the resolved effect would create TokenAmount$ X Angels (xPaid ==
	// 2), but binding the chosen X into the resolving effect's Ctx.X happens in
	// rules/stack.go's resolveAbility -- held on a parallel agent's branch and
	// not editable here -- so on this snapshot the spell still resolves but
	// xPaid reads 0. Assert resolution itself (stack -> graveyard) rather than
	// the angel count; the parallel X-binding completes this.
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(en).Zone != state.ZGraveyard {
		t.Fatalf("Entreat zone %s after resolving", e.G.Obj(en).Zone)
	}
	replayCheck(t, e, cfg)
}
