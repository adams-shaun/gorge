package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEDHDayTimeChangesCelestusLife plays The Celestus's printed text through
// a real game: it enters with neither day nor night (its Moved replacement
// makes it day), then its "{3}, {T}: If it's night, it becomes day. Otherwise,
// it becomes night" ability flips day to night and its Mode$ DayTimeChanges
// trigger gains 1 life.
func TestEDHDayTimeChangesCelestusLife(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := append([]*cards.Card{searchCorpusCard(t, reg, "The Celestus")}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 9432, Names: []string{"celestus", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	castVanilla(t, e, "The Celestus", "RRR")
	id := celestusOnBoard(t, e)
	if e.G.DayNight != state.DayNightDay {
		t.Fatalf("after The Celestus enters with neither: %q, want day", e.G.DayNight)
	}
	// Activate "{3}, {T}: If it's night, it becomes day. Otherwise, it becomes
	// night" while it is day.
	addMana(t, e, 0, "RRR")
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat 0's priority, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		// The {T} mana activation is Kind "activate"; the {3}, {T} DayTime
		// activation is Kind "ability" with its ability index.
		if o.Kind == "ability" && o.Obj == id && o.Ability != 0 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("precondition: The Celestus's DayTime ability not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// Drain the activation: the trigger's "You may draw a card. If you do,
	// discard a card." body poses the UnlessCost payer ask mid-resolution —
	// decline the draw, so the life gain is the only effect under test.
	answered := false
	for i := 0; i < 40 && len(e.G.Stack) > 0 && !e.G.Over; i++ {
		d2 := e.Pending()
		if d2 == nil {
			t.Fatalf("no decision while draining (stack depth %d)", len(e.G.Stack))
		}
		if d2.Kind == decision.KModes && d2.ResumeKind == "unless_pay" {
			decline := -1
			for _, o := range d2.Options {
				if o.Mode == "unless_decline" {
					decline = o.Index
				}
			}
			if decline < 0 {
				t.Fatalf("no decline option on the unless ask: %+v", d2.Options)
			}
			submitChoices(t, e, decline)
			answered = true
			continue
		}
		if d2.Kind != decision.KPriority {
			t.Fatalf("unexpected ask while draining: %+v", d2)
		}
		passIdx := -1
		for _, o := range d2.Options {
			if o.Kind == "pass" {
				passIdx = o.Index
			}
		}
		if passIdx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d2)
		}
		submitChoices(t, e, passIdx)
	}
	if !answered {
		t.Fatal("the trigger's optional-draw unless ask never came up")
	}
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("after the ability resolves: %q, want night", e.G.DayNight)
	}
	if life := e.G.Players[0].Life; life != 21 {
		t.Fatalf("life = %d after day became night, want 21 (trigger gains 1)", life)
	}
	replayCheck(t, e, cfg)
}

// TestEDHDayTimeChangesCelestusFirstDesignation pins the CR 731.2a gate: the
// game's FIRST designation (neither becoming day) is not a change of day to
// night or night to day, so The Celestus's trigger stays silent for it.
func TestEDHDayTimeChangesCelestusFirstDesignation(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := append([]*cards.Card{searchCorpusCard(t, reg, "The Celestus")}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 9433, Names: []string{"celestus", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	castVanilla(t, e, "The Celestus", "RRR")
	if e.G.DayNight != state.DayNightDay {
		t.Fatalf("after The Celestus enters with neither: %q, want day", e.G.DayNight)
	}
	if n := len(dayNightChanges(e)); n != 1 {
		t.Fatalf("day/night events = %d, want 1 (the designation itself)", n)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life = %d after neither became day, want 20 (no DayTimeChanges)", life)
	}
	replayCheck(t, e, cfg)
}

// celestusOnBoard returns the battlefield ID of the cast The Celestus.
func celestusOnBoard(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "The Celestus" {
			return id
		}
	}
	t.Fatal("The Celestus absent from seat 0's battlefield")
	return 0
}
