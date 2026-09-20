package rules

// CanRepeatModes$ True (CR 601.2b's "you may choose the same mode more than
// once"): the modal pick becomes an ordered multiset over the distinct
// Choices$ modes, so one mode can fill several of the CharmNum$ slots. The
// pin is Fiery Confluence -- the corpus carrier whose oracle is literally
// "Choose three. You may choose the same mode more than once." -- driven end
// to end through the cast-time announcement (castModeAsk), the cast's own
// record (ChosenModes), and resolution's effCharm re-run. Three picks of the
// damage-opponents mode must deal 2 damage to each opponent three times.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// fieryConfluenceEngine seeds the REAL corpus Fiery Confluence into seat 0's
// deck (so genesis creates it and the log replays byte-for-byte), bridges it
// into hand, and returns the engine, the Config its log replays against, and
// the card's id. This mirrors newFixtureDeck's fixture-seeding path; the card
// itself comes from the corpus so the exact shipped parameters are what the
// engine defends.
func fieryConfluenceEngine(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	fiery, ok := testutil.CorpusRegistry(t).Lookup("Fiery Confluence")
	if !ok {
		t.Fatal("corpus missing Fiery Confluence")
	}
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{fiery}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: map[string]*cards.Card{},
	}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()

	var id state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(cand).Face().Name == "Fiery Confluence" {
			id = cand
		}
	}
	if id == 0 {
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == "Fiery Confluence" {
				id = cand
			}
		}
		if id == 0 {
			t.Fatal("Fiery Confluence not found in seat 0's hand or library")
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}
	return e, cfg, id
}

// TestFieryConfluenceRepeatsAMode pins CanRepeatModes$ True end to end on the
// corpus card. The engine must offer the repeatable KModes (Repeatable true,
// Min == Max == CharmNum 3), accept the SAME option index three times
// (Decision.Validate's no-duplicate rule relaxed only for this ask), and run
// the mode three times at resolution.
func TestFieryConfluenceRepeatsAMode(t *testing.T) {
	e, cfg, id := fieryConfluenceEngine(t, 1)
	addMana(t, e, 0, "RRRR") // 2 R R

	// Cast through the priority window's own offer so castModeAsk runs.
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	if !d.Repeatable {
		t.Fatal("Fiery Confluence's cast-mode ask is not Repeatable -- CanRepeatModes$ True is unread")
	}
	if d.Min != 3 || d.Max != 3 {
		t.Fatalf("mode bounds = %d..%d, want 3..3 (CharmNum$ 3)", d.Min, d.Max)
	}
	// DBDestroy (ValidTgts$ Artifact) has no legal target on this empty
	// board, so only the two targetless damage modes are offered -- and the
	// repeatable ask must still allow all three slots to be filled.
	if len(d.Options) != 2 {
		t.Fatalf("eligible mode options = %d, want 2 (destroy-artifact mode has no target): %+v",
			len(d.Options), d.Options)
	}
	opp := -1
	for _, o := range d.Options {
		if o.Label == "CARDNAME deals 2 damage to each opponent." {
			opp = o.Index
		}
	}
	if opp < 0 {
		t.Fatalf("damage-opponents mode not offered: %+v", d.Options)
	}

	// The repeated pick: the SAME index three times. Pre-fix this intent is
	// rejected by Validate ("duplicate choice"), which is exactly the bug.
	start := e.G.Players[1].Life
	submitChoices(t, e, opp, opp, opp)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Players[1].Life; got != start-6 {
		t.Fatalf("opponent life = %d, want %d (the mode must resolve three times, 2 damage each)", got, start-6)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Errorf("Fiery Confluence resolved to %s, want Graveyard", z)
	}
	// The ModeChosen marker records the three picks in answer order --
	// duplicates preserved -- and replay re-derives them byte-for-byte.
	labels := modeChosenLabels(t, e)
	if len(labels) != 3 || labels[0] != labels[1] || labels[1] != labels[2] {
		t.Fatalf("ModeChosen = %v, want the same mode label three times", labels)
	}
	replayCheck(t, e, cfg)
}

// modeChosenLabels recovers the most recent ModeChosen payload (the option
// labels, comma-joined) from the log for the assertion above.
func modeChosenLabels(t *testing.T, e *Engine) []string {
	t.Helper()
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.ModeChosen && ev.Text != "" {
			return strings.Split(ev.Text, ",")
		}
	}
	return nil
}
