package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// These fixtures pin api:Clash and trig:Clashed (CR 701.31, task clash1) on the
// real corpus carrier Marvo, Deep Operative, plus the trigger mode's own
// Won$ orientation gate.
//
// Marvo's attack trigger is `DB$ Clash | Defined$ TriggeredDefendingPlayer`
// and its second line is `T:Mode$ Clashed | ValidPlayer$ You | Won$ True`
// (draw a card, then may cast an MV<=8 spell for free). The clash therefore
// runs entirely inside the attack trigger's resolution: seat 0 (Marvo's
// controller) reveals the top of its library, seat 1 (the defending player)
// reveals its own, the higher mana value wins, and the Clashed marker the
// clash emits fires Marvo's own "whenever you win a clash" line.

// hasClashEvent reports whether the log holds one events.Clash marker for
// player p with the given Won$ orientation (Amount != 0 means won).
func hasClashEvent(e *Engine, p state.PlayerID, won bool) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Clash && ev.Player == p {
			if (ev.Amount != 0) == won {
				return true
			}
		}
	}
	return false
}

// TestMarvoDeepOperativeClashWinsDrawsAndOffersFreeCast drives the full
// attack -> clash -> win -> draw chain on the real card. Preconditions the
// mechanic reads are asserted first (Marvo on the battlefield; the two
// libraries' tops are the cards the comparison is about; seat 1 is alive), so
// a vacuous setup fails loudly.
func TestMarvoDeepOperativeClashWinsDrawsAndOffersFreeCast(t *testing.T) {
	e := combatEngine(t)
	marvo := onBoardCard(t, e, 0, unblockedCorpusCard(t, "m/marvo_deep_operative.txt"))
	e.G.Obj(marvo).SummonSick = false

	high := putTopOfLibrary(t, e, card(t, "Name:Huge Beast\nManaCost:5 G\nTypes:Creature Beast\nPT:5/5\nOracle:x\n"), 0)
	low := putTopOfLibrary(t, e, card(t, "Name:Tiny Beast\nManaCost:0\nTypes:Creature Beast\nPT:1/1\nOracle:x\n"), 1)

	// Preconditions: the attacker exists and is able to attack, both library
	// tops are the cards under test, and the defending seat is live.
	if o := e.G.Obj(marvo); o == nil || o.Zone != state.ZBattlefield || o.SummonSick {
		t.Fatalf("Marvo precondition: %+v", e.G.Obj(marvo))
	}
	if lib0 := e.G.Zone(state.ZLibrary, 0); len(lib0) == 0 || lib0[0] != high {
		t.Fatalf("seat 0 library top precondition: %v, want high card %d", lib0, high)
	}
	if lib1 := e.G.Zone(state.ZLibrary, 1); len(lib1) == 0 || lib1[0] != low {
		t.Fatalf("seat 1 library top precondition: %v, want low card %d", lib1, low)
	}
	if e.G.Players[1].Lost {
		t.Fatal("seat 1 precondition: defending player is already lost")
	}
	hand0 := len(e.G.Zone(state.ZHand, 0))

	// Marvo attacks; its attack trigger is queued. The declaration goes
	// through the engine's own emit so TriggeredDefendingPlayer is captured --
	// the attacker batched against seat 1, so the event's Player is the
	// DEFENDING seat (the role rules/trigger_referents.go binds
	// DefendingPlayer to), exactly as a real declaration emits it.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{marvo}})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("Marvo's attack trigger was not queued by the declaration")
	}
	e.resolveTop()

	// The clash ran: the unimplemented-API fallback must be ABSENT, and seat
	// 0's win must be recorded.
	if hasNote(e, "unimplemented API Clash") {
		t.Fatal("api:Clash resolved through the unregistered fallback Note, not the registered handler")
	}
	if !hasClashEvent(e, 0, true) {
		t.Fatal("no Clash marker recording seat 0's win")
	}
	if !hasClashEvent(e, 1, false) {
		t.Fatal("no Clash marker recording seat 1's loss")
	}
	// The placement stand-in: both revealed cards are on the bottom of their
	// owner's library.
	if lib0 := e.G.Zone(state.ZLibrary, 0); len(lib0) == 0 || lib0[len(lib0)-1] != high {
		t.Fatalf("seat 0 revealed card %d is not on the bottom of its library: %v", high, lib0)
	}
	if lib1 := e.G.Zone(state.ZLibrary, 1); len(lib1) == 0 || lib1[len(lib1)-1] != low {
		t.Fatalf("seat 1 revealed card %d is not on the bottom of its library: %v", low, lib1)
	}

	// The Clashed marker fires Marvo's `Won$ True` line: draw a card, then
	// offer the optional free cast. drainProvTriggers stops on that ask.
	drainProvTriggers(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand0+1 {
		t.Fatalf("seat 0 hand = %d after winning the clash, want %d (the Won$ True draw)", got, hand0+1)
	}
}

// TestClashPrimitivesRegistered pins that both halves of the mechanic are
// declared supported, so the coverage census counts the 29 api:Clash and the 4
// trig:Clashed corpus carriers as playable. Reverting either registration is a
// silent coverage regression this fails on.
func TestClashPrimitivesRegistered(t *testing.T) {
	supported := effects.Supported()
	for _, p := range []string{"api:Clash", "trig:Clashed"} {
		if !supported[p] {
			t.Fatalf("effects.Supported() is missing %s", p)
		}
	}
}

// TestClashTriggerModeReadsWonOrientation pins the two parameters the corpus's
// Clashed lines actually carry -- ValidPlayer$ and the Won$ True/False split
// (Entangling Trap and Rebellion of the Flamekin each carry a True line and a
// Secondary$ True False sibling). It exercises the matcher directly, so it
// fails if the Clashed registration is removed, and it asserts the NEGATIVE
// answers too: a Won$ True line must reject a loss and a ValidPlayer$ You line
// must reject the other seat.
func TestClashTriggerModeReadsWonOrientation(t *testing.T) {
	e := layerEngine(t)
	const src = state.ObjID(1) // any id: controllerOf degrades to 0 for it

	winTrue := cards.Trigger{Mode: "Clashed", Params: map[string]string{"ValidPlayer": "You", "Won": "True"}}
	winFalse := cards.Trigger{Mode: "Clashed", Params: map[string]string{"ValidPlayer": "You", "Won": "False"}}

	win0 := events.Event{Kind: events.Clash, Obj: src, Player: 0, Amount: 1}
	loss0 := events.Event{Kind: events.Clash, Obj: src, Player: 0, Amount: 0}
	win1 := events.Event{Kind: events.Clash, Obj: src, Player: 1, Amount: 1}

	// Precondition: the two orientations really differ, so the assertions
	// below are about the gate and not about two identical events.
	if (win0.Amount != 0) == (loss0.Amount != 0) {
		t.Fatal("test precondition: win and loss events carry the same amount")
	}

	if !e.clashMatches(winTrue, src, win0, nil) {
		t.Error("Won$ True line rejected the matching win")
	}
	if e.clashMatches(winTrue, src, loss0, nil) {
		t.Error("Won$ True line accepted a loss")
	}
	if e.clashMatches(winTrue, src, win1, nil) {
		t.Error("ValidPlayer$ You line accepted another seat's win")
	}
	if !e.clashMatches(winFalse, src, loss0, nil) {
		t.Error("Won$ False line rejected the matching loss")
	}
	if e.clashMatches(winFalse, src, win0, nil) {
		t.Error("Won$ False line accepted a win")
	}
}
