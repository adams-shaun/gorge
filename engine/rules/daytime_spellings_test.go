package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDayTimeSpellings pins the three spellings that name the CR 731
// designation directly, over Brimstone Vandal: its Moved replacement's
// DayTime$ Neither gate runs DB$ DayTime | Value$ Day as it enters with
// neither day nor night, and its Mode$ DayTimeChanges trigger fires once
// when day becomes night (CR 731.2a) -- never for neither becoming day.
func TestDayTimeSpellings(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := append([]*cards.Card{searchCorpusCard(t, reg, "Brimstone Vandal")}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 9431, Names: []string{"vandal", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	castVanilla(t, e, "Brimstone Vandal", "RRR")
	if e.G.DayNight != state.DayNightDay {
		t.Fatalf("after Brimstone Vandal enters with neither: %q, want day", e.G.DayNight)
	}
	if n := len(dayNightChanges(e)); n != 1 {
		t.Fatalf("day/night events = %d, want 1", n)
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("opponent life = %d after neither became day, want 20 (no DayTimeChanges)", life)
	}
	// Turn 2: seat 0 cast a spell in turn 1, so it stays day. Turn 3's untap:
	// seat 1 cast nothing in turn 2, so it becomes night and the trigger
	// deals 1 damage to the opponent.
	driveToStep(t, e, 3, 0, state.StepDraw)
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("turn 3: %q, want night", e.G.DayNight)
	}
	if life := e.G.Players[1].Life; life != 19 {
		t.Fatalf("opponent life = %d after day became night, want 19", life)
	}
	replayCheck(t, e, cfg)
}
