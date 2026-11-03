package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// limitedWatcherSrc is a creature whose "becomes the target of a spell"
// trigger fires at most once each turn.
const limitedWatcherSrc = "Name:Limited Watcher\nManaCost:2\nTypes:Creature Toy\nPT:1/1\n" +
	"T:Mode$ BecomesTarget | ValidTarget$ Card.Self | ValidSource$ Spell | TriggerZones$ Battlefield | ActivationLimit$ 1 | Execute$ TrigPing | TriggerDescription$ Each turn, the first time this is targeted by a spell, it pings each opponent.\n" +
	"SVar:TrigPing:DB$ LoseLife | Defined$ Opponent | LifeAmount$ 1\n" +
	"Oracle:x\n"

// TestReversedCastGivesBackOnceEachTurnTrigger: a cast whose target choice
// queued a once-each-turn trigger and was then reversed (CR 733.1) has
// triggered nothing, so the trigger has not used its turn. The second, legal
// cast of the same spell must still fire it.
func TestReversedCastGivesBackOnceEachTurnTrigger(t *testing.T) {
	t.Parallel()
	phalanx := choiceCorpusCard(t, "Phalanx Formation")
	deck := []*cards.Card{phalanx, card(t, limitedWatcherSrc), card(t, gateRaiderSrc), card(t, gateRaiderSrc)}
	cfg := seatZeroStart(Config{Seed: 953, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(deck, mountainDeck(t, 40-len(deck))...),
			mountainDeck(t, 40),
		},
		Tokens: map[string]*cards.Card{},
	})
	e := New(cfg)
	e.Advance()
	pf := findInZones(t, e, 0, "Phalanx Formation")
	if inZone(e, state.ZLibrary, 0, pf) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: pf, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}
	toMain1(t, e)
	watcher := gateMoveFromLibrary(t, e, "Limited Watcher", state.ZBattlefield)
	r1 := gateMoveFromLibrary(t, e, "Raider", state.ZBattlefield)
	gateMoveFromLibrary(t, e, "Raider", state.ZBattlefield)
	addMana(t, e, 0, "WWW")

	// Targeting the watcher and a Raider prices one Strive payment too many
	// for the pool: the cast reverses.
	submitChoices(t, e, striveCastOption(t, e, pf).Index)
	chooseStriveTargets(t, e, watcher, r1)
	if o := e.G.Obj(pf); o.Zone != state.ZHand {
		t.Fatalf("the unpayable cast left Phalanx Formation in %v, want hand", o.Zone)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("the reversed cast left %d queued trigger(s)", len(e.pendingTriggers))
	}

	// The legal cast targets only the watcher; it must still trigger.
	submitChoices(t, e, striveCastOption(t, e, pf).Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the target ask, got %+v", d)
	}
	chooseStriveTargets(t, e, watcher)
	pushes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == watcher {
			pushes++
		}
	}
	if pushes+len(e.pendingTriggers) != 1 {
		t.Fatalf("the legal cast fired the trigger %d time(s), want 1: the reversed one must not have used the turn", pushes+len(e.pendingTriggers))
	}
}
