package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// ringEmblemEngine is layerEngine plus the Config its replay needs.
func ringEmblemEngine(t *testing.T) (*Engine, Config) {
	t.Helper()
	cfg := seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	return New(cfg), cfg
}

// countLifeChange sums the LifeChange events for one seat (the emblem's level
// 4 drain is a LoseLife -> LifeChange with a negative amount).
func countLifeChange(e *Engine, p state.PlayerID) int32 {
	var total int32
	for _, ev := range e.L.Events {
		if ev.Kind == events.LifeChange && ev.Player == p {
			total += ev.Amount
		}
	}
	return total
}

// temptSeat emits one RingTemptsYou for p with the given bearer and drains
// nothing -- the caller does. Used to build a temptation count without the
// Call of the Ring payoff trigger complicating the queue.
func temptSeat(e *Engine, p state.PlayerID, bearer state.ObjID) {
	e.emit(events.Event{Kind: events.RingTemptsYou, Player: p, Obj: bearer,
		Amount: e.G.Players[p].RingTempted + 1})
}

// TestRingEmblemLevel2IgnoresNonBearerAttacks pins that level 2 names the
// Ring-bearer, not "a creature you control": a second attacker that is not the
// bearer does not loot, and the bearer must be the one in the attacker list.
func TestRingEmblemLevel2IgnoresNonBearerAttacks(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	other := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	temptSeat(e, 0, bear)
	temptSeat(e, 0, bear) // level 2

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{other}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("a non-bearer attacker queued %d emblem triggers, want none", len(e.G.Stack))
	}
}

// TestRingEmblemLevel4NonCombatDamageDoesNotDrain pins that level 4 names
// COMBAT damage: the identical Damage event with the combat flag unset (an
// ordinary burn spell) fires nothing.
func TestRingEmblemLevel4NonCombatDamageDoesNotDrain(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	for i := 0; i < 4; i++ {
		temptSeat(e, 0, bear)
	}

	e.damaging = bear
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.damaging = 0
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("non-combat damage queued %d emblem triggers, want none", len(e.G.Stack))
	}
}

// TestRingEmblemAbilitiesReplayByteIdentically drives every triggered level
// ability and then replays the recorded log through events.Apply alone.
// The emblem's stack objects are minted by the RingEmblemPush event inside
// events.Apply (Ruling T20-a), so a log-only replay reconstructs the
// identical game state -- no unlogged Game.AddObject names an ObjID the
// replay never learns about.
//
// The baseline is a clone taken just before the emblem events: the fixture's
// board placement (onBoardCard) is deliberately eventless, so replayFromLog's
// genesis would not carry it and every object id would shift. Cloning keeps
// the ids the log was written against, which is the property under test.
func TestRingEmblemAbilitiesReplayByteIdentically(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	blocker := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))

	base := e.G.Clone()
	mark := len(e.L.Events)

	// Temptations 1..4: every level becomes live; a temptation itself
	// triggers nothing.
	for i := 0; i < 4; i++ {
		temptSeat(e, 0, bear)
	}

	// Level 2: the bearer attacks, draws, and discards (answer the ask).
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	e.resolveTop()
	if e.Pending() != nil {
		submitChoices(t, e, 0)
	}

	// Level 3: the bearer becomes blocked; the blocker is sacrificed when
	// the end of combat step begins (after the damage below).
	e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1,
		Pairs: [][2]state.ObjID{{bear, blocker}}})
	e.putTriggersOnStack()
	e.resolveTop()

	// Level 4: the bearer deals combat damage to a player; each opponent
	// loses 3 life.
	e.damaging = bear
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	e.putTriggersOnStack()
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEndCombat})
	e.putTriggersOnStack()
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	if o := e.G.Obj(blocker); o.Zone != state.ZGraveyard {
		t.Fatalf("level 3: blocker zone %v after end of combat, want graveyard", o.Zone)
	}

	if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.RingEmblemPush }); n != 3 {
		t.Fatalf("RingEmblemPush events = %d, want 3 (one per triggered level)", n)
	}

	replayed := base.Clone()
	for _, ev := range e.L.Events[mark:] {
		events.Apply(replayed, ev)
	}
	if diff := diffGames(e.G, replayed); diff != "" {
		t.Fatalf("log-only replay of the emblem events differs:\n%s", diff)
	}
}

// TestRingEmblemLevelsRequireALiveBearer pins that the triggered levels name
// "your Ring-bearer": a seat tempted with no creature (Obj 0, CR 701.54d) has
// a count but no bearer, so attacking, being blocked and combat damage fire
// nothing.
func TestRingEmblemLevelsRequireALiveBearer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	blocker := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	// No creature was available at tempt time: count 4, bearer 0.
	for i := 0; i < 4; i++ {
		temptSeat(e, 0, 0)
	}
	if e.G.Players[0].RingBearer != 0 {
		t.Fatalf("fixture: bearer %d, want 0", e.G.Players[0].RingBearer)
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1,
		Pairs: [][2]state.ObjID{{bear, blocker}}})
	e.damaging = bear
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("a seat with no Ring-bearer queued %d emblem triggers, want none", len(e.G.Stack))
	}
}

// TestRingEmblemLevel4DrainsEvenWhenTheBearerDied pins CR 701.54c's level 4
// when the Ring-bearer dies in the same combat: the ability triggered on the
// combat damage, and its body names no bearer, so it still resolves and each
// opponent loses 3 life with no decision posed.
func TestRingEmblemLevel4DrainsEvenWhenTheBearerDied(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	for i := 0; i < 4; i++ {
		temptSeat(e, 0, bear)
	}

	e.damaging = bear
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	// Combat damage is dealt simultaneously: the bearer dies before the
	// triggered ability resolves (CR 704.5g), clearing the designation.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield,
		To: state.ZGraveyard})
	if e.G.Players[0].RingBearer != 0 {
		t.Fatalf("fixture: the dead bearer is still designated (%d)", e.G.Players[0].RingBearer)
	}
	before := countLifeChange(e, 1)
	e.putTriggersOnStack()
	if e.Pending() != nil {
		t.Fatalf("level 4 with a dead bearer posed a decision %+v, want none", e.Pending())
	}
	e.resolveTop()
	if got := before - countLifeChange(e, 1); got != 3 {
		t.Fatalf("level 4 with a dead bearer drained %d, want 3", got)
	}
}

// TestRingEmblemPendingTriggerIntegration pins the two queue-side touch
// points the emblem entries rely on: triggerLabel (read by an ordering ask
// and the optional-trigger prompt) names the level without a face to read
// from, and optionalDecider reports the level abilities as mandatory
// (CR 701.54c's "whenever" abilities have no OptionalDecider$ and no face for
// triggerOf to walk). A coexistence ordering ask is therefore safe.
func TestRingEmblemPendingTriggerIntegration(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	for _, lvl := range []int{ringEmblemLevelLoot, ringEmblemLevelBlockedBy, ringEmblemLevelCombatHit} {
		pt := pendingTrigger{RingEmblem: lvl}
		if got := e.triggerLabel(pt); got == "" || got == ringEmblemLabel(0) {
			t.Fatalf("level %d has no level-specific trigger label (%q)", lvl, got)
		}
		who, optional, askable := e.optionalDecider(pt)
		if optional || askable {
			t.Fatalf("level %d reported optional=%v askable=%v, want mandatory",
				lvl, optional, askable)
		}
		_ = who
	}
}
