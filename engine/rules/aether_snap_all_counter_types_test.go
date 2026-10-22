package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAetherSnapRemovesAllCounters pins the corpus Aether Snap script's
// `AllCounterTypes$ True` shape: with no CounterType$ it must remove EVERY
// counter kind from each permanent (CR 122.8), not return early and leave the
// board untouched. The internal Shield marker is not a counter (cr-g83), so it
// stays, and the SubAbility still exiles the token.
func TestAetherSnapRemovesAllCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	snap := mustCorpusCard(t, reg, "Aether Snap")
	bearSrc := "Name:Countered Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	cfg := seatZeroStart(Config{Seed: 170, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{snap, card(t, bearSrc)}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()

	snapID := moveSeededCard(t, e, 0, snap, state.ZHand)
	bear := moveSeeded(t, e, 0, bearSrc, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "CHARGE", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "Shield", Amount: 1})
	token := putToken(t, e, 0, "Name:Snap Token\nTypes:Creature\nPT:1/1\nOracle:x\n", state.ZBattlefield)

	// Preconditions: the permanent really carries both counter kinds and the
	// token really is a battlefield permanent, so the assertions below are
	// about Aether Snap's sweep rather than an empty board.
	if got := e.G.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: P1P1 = %d, want 2", got)
	}
	if got := e.G.Obj(bear).Counter("CHARGE"); got != 1 {
		t.Fatalf("precondition: CHARGE = %d, want 1", got)
	}
	if e.G.Obj(token).Zone != state.ZBattlefield {
		t.Fatalf("precondition: token zone = %s, want battlefield", e.G.Obj(token).Zone)
	}

	addMana(t, e, 0, "BBBBB")
	submitChoices(t, e, castOptionFor(t, e, snapID).Index)
	passUntilStackEmpty(t, e, 30)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 = %d, want 0 (AllCounterTypes$ True removes every kind)", got)
	}
	if got := e.G.Obj(bear).Counter("CHARGE"); got != 0 {
		t.Fatalf("CHARGE = %d, want 0 (AllCounterTypes$ True removes every kind)", got)
	}
	if got := e.G.Obj(bear).Counter("Shield"); got != 1 {
		t.Fatalf("Shield = %d, want 1 (the regeneration shield is not a counter)", got)
	}
	if e.G.Obj(token).Zone == state.ZBattlefield {
		t.Fatal("token stayed on the battlefield after Aether Snap")
	}
	if !exileMoveLanded(e, token) {
		t.Fatal("no MoveZone to Exile recorded for the token")
	}
	replayCheck(t, e, cfg)
}
