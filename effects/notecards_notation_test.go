package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func notationSource(t *testing.T, h *fakeHost, card *cards.Card) state.ObjID {
	t.Helper()
	o := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

// TestPlayerNotedIsReplayableSharedPlayerState keeps the write event-backed
// and pins the shared player filter reader used by RepeatPlayers$ and every
// other Player.NotedFor<label> consumer.
func TestPlayerNotedIsReplayableSharedPlayerState(t *testing.T) {
	h := newHost(t, 3)
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 2, Text: "Fortune"})

	if got := h.g.Players[1].Notes; len(got) != 1 || got[0] != "Fame" {
		t.Fatalf("player 1 notes = %v, want [Fame] (event write must be deduplicated)", got)
	}
	if !MatchesPlayerSpecFrom(h.g, "Player.NotedForFame", 1, 0, 0) ||
		MatchesPlayerSpecFrom(h.g, "Player.NotedForFame", 2, 0, 0) ||
		!MatchesPlayerSpecFrom(h.g, "Player.NotedForFortune", 2, 0, 0) {
		t.Fatalf("notation filter did not split Fame/Fortune: p1=%v p2=%v", h.g.Players[1].Notes, h.g.Players[2].Notes)
	}
	clone := h.g.Clone()
	clone.Players[1].Notes = append(clone.Players[1].Notes, "Other")
	if len(h.g.Players[1].Notes) != 1 {
		t.Fatalf("clone aliased player notes: live notes = %v", h.g.Players[1].Notes)
	}
}

// TestSeizeTheSpotlightTwoOpponentSplit drives the real corpus SVar bodies:
// one remembered chooser takes Fame and another Fortune, and the card's own
// RepeatEach selectors walk only their corresponding notation group.
func TestSeizeTheSpotlightTwoOpponentSplit(t *testing.T) {
	card, fame := corpusSA(t, "Seize the Spotlight", "Fame")
	_, fortune := corpusSA(t, "Seize the Spotlight", "Fortune")
	_, fameLoop := corpusSA(t, "Seize the Spotlight", "DBFame")
	_, fortuneLoop := corpusSA(t, "Seize the Spotlight", "DBFortune")
	if got := fame.Params["NoteCards"]; got != "Self" {
		t.Fatalf("Fame NoteCards$ = %q, want Self", got)
	}
	if got := fame.Params["NoteCardsFor"]; got != "Fame" {
		t.Fatalf("Fame NoteCardsFor$ = %q, want Fame", got)
	}
	if got := fameLoop.Params["RepeatPlayers"]; got != "Player.NotedForFame" {
		t.Fatalf("DBFame RepeatPlayers$ = %q, want Player.NotedForFame", got)
	}
	if got := fortuneLoop.Params["RepeatPlayers"]; got != "Player.NotedForFortune" {
		t.Fatalf("DBFortune RepeatPlayers$ = %q, want Player.NotedForFortune", got)
	}
	if got := fameLoop.Params["ClearRememberedBeforeLoop"]; got != "True" {
		t.Fatalf("DBFame ClearRememberedBeforeLoop$ = %q, want True", got)
	}

	h := newHost(t, 3)
	src := notationSource(t, h, card)
	svars := h.g.Obj(src).Face().SVars
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 1, IsPlayer: true}}}, fame)
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 2, IsPlayer: true}}}, fortune)
	if got := h.g.Players[1].Notes; len(got) != 1 || got[0] != "Fame" {
		t.Fatalf("fame chooser notes = %v, want [Fame]", got)
	}
	if got := h.g.Players[2].Notes; len(got) != 1 || got[0] != "Fortune" {
		t.Fatalf("fortune chooser notes = %v, want [Fortune]", got)
	}

	// Substitute observable bodies while retaining the corpus loops and their
	// selectors. Each loop binds its subject as Remembered, so it loses life.
	bodies := map[string]string{
		"GainControl": "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1",
		"DBDraw":      "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1",
	}
	effRepeatEach(h, &Ctx{Source: src, Controller: 0, SVars: bodies}, fameLoop)
	if h.g.Players[1].Life != 19 || h.g.Players[2].Life != 20 {
		t.Fatalf("fame loop lives = [%d %d], want [19 20]", h.g.Players[1].Life, h.g.Players[2].Life)
	}
	effRepeatEach(h, &Ctx{Source: src, Controller: 0, SVars: bodies}, fortuneLoop)
	if h.g.Players[1].Life != 19 || h.g.Players[2].Life != 19 {
		t.Fatalf("fortune loop lives = [%d %d], want [19 19]", h.g.Players[1].Life, h.g.Players[2].Life)
	}
}

// TestRepeatEachClearRememberedBeforeLoop proves the loop hygiene flag clears
// only after resolving its subject selector, before the first body runs.
func TestRepeatEachClearRememberedBeforeLoop(t *testing.T) {
	card, loop := corpusSA(t, "Seize the Spotlight", "DBFame")
	h := newHost(t, 2)
	src := notationSource(t, h, card)
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	chaff := h.g.AddObject(mkCard(t, "Name:Chaff\nTypes:Sorcery\nOracle:x\n"), 0)
	c := &Ctx{Source: src, Controller: 0,
		SVars:      map[string]string{"GainControl": "DB$ LoseLife | Defined$ You | LifeAmount$ 1"},
		Remembered: []state.Target{{Obj: chaff.ID}}}
	effRepeatEach(h, c, loop)
	if h.g.Players[0].Life != 19 { // precondition: the selector found Fame.
		t.Fatalf("loop did not run: controller life = %d, want 19", h.g.Players[0].Life)
	}
	if len(c.Remembered) != 0 {
		t.Fatalf("remembered after clearing loop = %v, want empty", c.Remembered)
	}
}
