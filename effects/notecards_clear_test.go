package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestCardNotationDoesNotWritePlayerLabels keeps NoteCards$ Remembered on
// Volatile Chimera in the card-notation half of the family. In particular, it
// must not make the resolving controller Player.NotedForVolatileChimera.
func TestCardNotationDoesNotWritePlayerLabels(t *testing.T) {
	card, note := corpusSA(t, "Volatile Chimera", "DBPump")
	if got := note.Params["NoteCards"]; got != "Remembered" {
		t.Fatalf("Volatile Chimera NoteCards$ = %q, want Remembered", got)
	}
	if got := note.Params["NoteCardsFor"]; got != "VolatileChimera" {
		t.Fatalf("Volatile Chimera NoteCardsFor$ = %q, want VolatileChimera", got)
	}

	h := newHost(t, 2)
	src := notationSource(t, h, card)
	if got := h.g.Obj(src).Zone; got != state.ZBattlefield {
		t.Fatalf("notation source zone = %v, want battlefield", got)
	}
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: h.g.Obj(src).Face().SVars}, note)
	if got := h.g.Players[0].Notes; len(got) != 0 {
		t.Fatalf("card notation wrote player labels = %v, want none", got)
	}
	for _, e := range h.log {
		if e.Kind == events.PlayerNoted {
			t.Fatalf("card notation emitted PlayerNoted event = %+v", e)
		}
	}
}

// TestClearNotedCardsForReplacesPriorChoice drives Master of Ceremonies'
// choice bodies on two upkeep-equivalent passes. The cleanup must make a
// changed Money choice become Friends only, including after log replay and on
// an isolated state clone.
func TestClearNotedCardsForReplacesPriorChoice(t *testing.T) {
	card, money := corpusSA(t, "Master of Ceremonies", "Money")
	_, friends := corpusSA(t, "Master of Ceremonies", "Friends")
	_, cleanup := corpusSA(t, "Master of Ceremonies", "DBCleanup")
	if got := cleanup.Params["ClearNotedCardsFor"]; got != "Money,Friends,Secrets" {
		t.Fatalf("Master cleanup ClearNotedCardsFor$ = %q, want Money,Friends,Secrets", got)
	}

	h := newHost(t, 2)
	src := notationSource(t, h, card)
	if got := h.g.Obj(src).Zone; got != state.ZBattlefield {
		t.Fatalf("Master source zone = %v, want battlefield", got)
	}
	ctx := func(remembered []state.Target) *Ctx {
		return &Ctx{Source: src, Controller: 0, SVars: h.g.Obj(src).Face().SVars, Remembered: remembered}
	}
	effPump(h, ctx([]state.Target{{Player: 1, IsPlayer: true}}), money)
	if !MatchesPlayerSpecFrom(h.g, "Player.NotedForMoney", 1, 0, src) {
		t.Fatal("Money choice did not note player 1")
	}
	effPump(h, ctx(nil), cleanup)
	if MatchesPlayerSpecFrom(h.g, "Player.NotedForMoney", 1, 0, src) {
		t.Fatal("cleanup retained player 1's prior Money label")
	}
	effPump(h, ctx([]state.Target{{Player: 1, IsPlayer: true}}), friends)
	if MatchesPlayerSpecFrom(h.g, "Player.NotedForMoney", 1, 0, src) ||
		!MatchesPlayerSpecFrom(h.g, "Player.NotedForFriends", 1, 0, src) {
		t.Fatalf("second choice labels = %v, want Friends only", h.g.Players[1].Notes)
	}

	replayed := state.NewGame(names(2))
	for _, e := range h.log {
		if e.Kind == events.PlayerNoted || e.Kind == events.PlayerNoteCleared {
			events.Apply(replayed, e)
		}
	}
	if !MatchesPlayerSpecFrom(replayed, "Player.NotedForFriends", 1, 0, 0) {
		t.Fatalf("replayed labels = %v, want Friends", replayed.Players[1].Notes)
	}
	clone := replayed.Clone()
	events.Apply(clone, events.Event{Kind: events.PlayerNoteCleared, Player: 1, Text: "Friends"})
	if !MatchesPlayerSpecFrom(replayed, "Player.NotedForFriends", 1, 0, 0) ||
		MatchesPlayerSpecFrom(clone, "Player.NotedForFriends", 1, 0, 0) {
		t.Fatalf("clear aliased replayed and cloned notes: replayed=%v clone=%v", replayed.Players[1].Notes, clone.Players[1].Notes)
	}
}
