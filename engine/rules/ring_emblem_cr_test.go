package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.54c, the Ring emblem's triggered levels, pinned on the real corpus
// tempt source Birthday Escape ("Draw a card. The Ring tempts you."):
//
//	2  "Whenever your Ring-bearer attacks, draw a card, then discard a card."
//	4  "Whenever your Ring-bearer deals combat damage to a player, each
//	   opponent loses 3 life."

// temptByBirthdayEscape resolves seat p's Birthday Escape n times straight off
// the stack (the putOppSpellOnStack direct-state licence) and drains whatever
// the temptations queued. With exactly one creature on p's side the
// Ring-bearer choice is silent (effRingTemptsYou's strict-supersets rule).
func temptByBirthdayEscape(t *testing.T, e *Engine, reg *cards.Registry, p state.PlayerID, n int) {
	t.Helper()
	card := mustCorpusCard(t, reg, "Birthday Escape")
	want := e.G.Players[p].RingTempted + int32(n)
	for i := 0; i < n; i++ {
		o := e.G.AddObject(card, p)
		o.Zone = state.ZStack
		e.G.SetZone(state.ZStack, p, append(e.G.Zone(state.ZStack, p), o.ID))
		e.resolveTop()
		if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
			t.Fatalf("temptation %d posed %v, want none (one creature: silent bearer)", i+1, d.Kind)
		}
		e.putTriggersOnStack()
		for len(e.G.Stack) > 0 {
			e.resolveTop()
		}
	}
	if got := e.G.Players[p].RingTempted; got != want {
		t.Fatalf("RingTempted = %d after %d more Birthday Escapes, want %d", got, n, want)
	}
}

func ringEmblemPushes(e *Engine) int {
	return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.RingEmblemPush })
}

// TestCR70154cRingLevel2AttackLoots: at one temptation an attack queues
// nothing; at two, the Ring-bearer attacking draws a card and then asks its
// controller to discard one.
func TestCR70154cRingLevel2AttackLoots(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))

	temptByBirthdayEscape(t, e, reg, 0, 1)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("one temptation: the Ring-bearer attacking queued %d abilities, want none", len(e.G.Stack))
	}

	temptByBirthdayEscape(t, e, reg, 0, 1)
	draws, discards := countDraws(e, 0), countEvents(e, events.IsDiscard)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("two temptations: attacking queued %d abilities, want the loot", len(e.G.Stack))
	}
	e.resolveTop()
	if n := countDraws(e, 0) - draws; n != 1 {
		t.Fatalf("level 2: drew %d, want 1", n)
	}
	if d := e.Pending(); d == nil || d.ResumeKind != "discard" || d.Player != 0 {
		t.Fatal("level 2: no discard ask for seat 0 after the draw")
	}
	submitChoices(t, e, 0)
	if n := countEvents(e, events.IsDiscard) - discards; n != 1 {
		t.Fatalf("level 2: discarded %d, want 1", n)
	}
}

// TestCR70154cRingLevel4CombatDamageDrainsEachOpponent3: in a three-player
// game, four temptations queue no ability of their own, three temptations'
// worth of combat damage drains nothing, and at four the Ring-bearer's combat
// damage to one player makes EACH opponent lose 3 life; the bearer stays.
func TestCR70154cRingLevel4CombatDamageDrainsEachOpponent3(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}))
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))

	hit := func() {
		e.damaging, e.combatDamaging = bear, true
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
		e.damaging, e.combatDamaging = 0, false
	}

	temptByBirthdayEscape(t, e, reg, 0, 3)
	hit()
	e.putTriggersOnStack()
	if len(e.G.Stack) != 0 {
		t.Fatalf("three temptations: combat damage queued %d abilities, want none", len(e.G.Stack))
	}

	temptByBirthdayEscape(t, e, reg, 0, 1)
	if n := ringEmblemPushes(e); n != 0 {
		t.Fatalf("temptations alone pushed %d Ring emblem abilities, want none", n)
	}
	life1, life2 := e.G.Players[1].Life, e.G.Players[2].Life
	hit()
	life1 = e.G.Players[1].Life // after the 2 combat damage itself
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("level 4: combat damage to a player queued %d abilities, want 1", len(e.G.Stack))
	}
	e.resolveTop()
	if got := life1 - e.G.Players[1].Life; got != 3 {
		t.Fatalf("level 4: the damaged opponent lost %d from the emblem, want 3", got)
	}
	if got := life2 - e.G.Players[2].Life; got != 3 {
		t.Fatalf("level 4: the other opponent lost %d, want 3", got)
	}
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield {
		t.Fatalf("level 4: the Ring-bearer left the battlefield (zone %v)", o.Zone)
	}
}
