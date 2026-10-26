package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const dayNightPingSrc = "Name:Ping\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1 | Defined$ You | SpellDescription$ You gain 1 life.\nOracle:You gain 1 life.\n"

// dayNightGame is a two-seat game whose seat-0 deck holds the real corpus
// Brutal Cathar (daybound) and Graveyard Trespasser (nightbound) plus two
// authored one-mana instants; seat 0 takes turns 1, 3, 5 and seat 1 turns 2, 4.
func dayNightGame(t *testing.T) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	deck := []*cards.Card{
		searchCorpusCard(t, reg, "Brutal Cathar"),
		searchCorpusCard(t, reg, "Graveyard Trespasser"),
		card(t, dayNightPingSrc), card(t, dayNightPingSrc),
	}
	deck = append(deck, mountainDeck(t, 36)...)
	cfg := seatZeroStart(Config{Seed: 9403, Names: []string{"day", "night"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

func dayNightChanges(e *Engine) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.DayNightChange {
			out = append(out, ev)
		}
	}
	return out
}

func TestCR731DayboundPermanentMakesItDay(t *testing.T) {
	e, cfg := dayNightGame(t)
	if e.G.DayNight != state.DayNightNeither {
		t.Fatalf("a new game is %q, want neither (CR 731.1)", e.G.DayNight)
	}
	searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	driveToStep(t, e, 1, 0, state.StepEnd)
	if e.G.DayNight != state.DayNightDay {
		t.Fatalf("with a daybound permanent the game is %q, want day (CR 702.145d)", e.G.DayNight)
	}
	ch := dayNightChanges(e)
	if len(ch) != 1 || ch[0].Amount != 1 || ch[0].Text != "day" {
		t.Fatalf("day/night events = %+v, want exactly one day", ch)
	}
	replayCheck(t, e, cfg)
}

func TestCR731DayBecomesNightWhenThePreviousActivePlayerCastNothing(t *testing.T) {
	e, cfg := dayNightGame(t)
	searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	driveToStep(t, e, 2, 1, state.StepUpkeep)
	if e.G.DayNight != state.DayNightNight || e.G.PrevTurnSpells != 0 {
		t.Fatalf("turn 2 upkeep: %q after %d spells, want night after 0 (CR 731.2a)", e.G.DayNight, e.G.PrevTurnSpells)
	}
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("turn 3 upkeep: %q, want night (CR 731.2b needs two spells)", e.G.DayNight)
	}
	if n := len(dayNightChanges(e)); n != 2 {
		t.Fatalf("day/night events = %d, want 2 (day, night)", n)
	}
	replayCheck(t, e, cfg)
}

func TestCR731NightBecomesDayAfterTwoSpells(t *testing.T) {
	for _, spells := range []int{1, 2} {
		e, cfg := dayNightGame(t)
		searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
		driveToStep(t, e, 3, 0, state.StepMain1)
		if e.G.DayNight != state.DayNightNight {
			t.Fatalf("spells=%d: precondition: %q, want night", spells, e.G.DayNight)
		}
		for i := 0; i < spells; i++ {
			castVanilla(t, e, "Ping", "R")
		}
		passUntilTurn(t, e, 4)
		want := state.DayNightNight
		if spells == 2 {
			want = state.DayNightDay
		}
		if e.G.DayNight != want || int(e.G.PrevTurnSpells) != spells {
			t.Fatalf("spells=%d: %q after %d spells, want %q (CR 731.2b)", spells, e.G.DayNight, e.G.PrevTurnSpells, want)
		}
		replayCheck(t, e, cfg)
	}
}

func TestCR702_145gNightboundAloneMakesItNight(t *testing.T) {
	e, cfg := dayNightGame(t)
	id := searchMoveByName(t, e, "Graveyard Trespasser", state.ZHand)
	// The effects.applyTransformed shape: a bare FlipFace, then the move.
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	if f := e.G.Obj(id).Face(); f == nil || f.Name != "Graveyard Glutton" {
		t.Fatalf("precondition: face = %+v, want Graveyard Glutton", f)
	}
	e.checkStateBased()
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("with a nightbound permanent alone the game is %q, want night (CR 702.145g)", e.G.DayNight)
	}
	replayCheck(t, e, cfg)
}
