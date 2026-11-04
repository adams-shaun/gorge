package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// holodeckEngine builds a game where seat 0 has The Holodeck on the
// battlefield, a creature and a land in the sideboard, and an artifact card
// plus a non-permanent card in the graveyard.
func holodeckEngine(t *testing.T) (e *Engine, bears, forest, ring, bolt, deck state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	m := mustCorpusCard(t, reg, "Mountain")
	fill := make([]*cards.Card, 40)
	for i := range fill {
		fill[i] = m
	}
	hol := mustCorpusCard(t, reg, "The Holodeck")
	cfg := Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks:      [][]*cards.Card{append([]*cards.Card{hol, mustCorpusCard(t, reg, "Sol Ring"), mustCorpusCard(t, reg, "Lightning Bolt")}, fill[:37]...), fill},
		Sideboards: [][]*cards.Card{{mustCorpusCard(t, reg, "Grizzly Bears"), mustCorpusCard(t, reg, "Forest")}, nil}}
	e = New(cfg)
	e.Advance()
	toMain1(t, e)
	deck = moveByName(t, e, 0, "The Holodeck", state.ZBattlefield)
	ring = moveByName(t, e, 0, "Sol Ring", state.ZGraveyard)
	bolt = moveByName(t, e, 0, "Lightning Bolt", state.ZGraveyard)
	sb := e.G.Zone(state.ZSideboard, 0)
	if len(sb) != 2 {
		t.Fatalf("sideboard = %v, want 2 cards", sb)
	}
	return e, sb[0], sb[1], ring, bolt, deck
}

func optionsFor(e *Engine, id state.ObjID) []decision.Option {
	var out []decision.Option
	for _, o := range e.legalActions(0) {
		if o.Obj == id {
			out = append(out, o)
		}
	}
	return out
}

// The Holodeck (CR 400.11: "outside the game" is the sideboard): its
// controller may play a permanent card they own from the sideboard or the
// graveyard, once during each of their turns.
func TestHolodeckOffersSideboardAndGraveyard(t *testing.T) {
	e, bears, forest, ring, bolt, _ := holodeckEngine(t)
	e.G.Step, e.G.Active, e.G.Priority = state.StepMain1, 0, 0
	addMana(t, e, 0, "GG2")
	if len(optionsFor(e, bears)) == 0 {
		t.Errorf("sideboard creature is not offered")
	}
	if len(optionsFor(e, forest)) == 0 {
		t.Errorf("sideboard land is not offered")
	}
	if len(optionsFor(e, ring)) == 0 {
		t.Errorf("graveyard artifact is not offered")
	}
	if len(optionsFor(e, bolt)) != 0 {
		t.Errorf("graveyard instant is offered: %+v", optionsFor(e, bolt))
	}
	// the sideboard stays hidden from the opponent's options
	for _, o := range e.legalActions(1) {
		if o.Obj == bears || o.Obj == forest {
			t.Errorf("opponent offered the sideboard card: %+v", o)
		}
	}
}

// Casting from the sideboard uses the turn's one permission: the graveyard
// card is no longer offered afterwards.
func TestHolodeckSideboardPlayConsumesTurnLimit(t *testing.T) {
	e, bears, _, ring, _, _ := holodeckEngine(t)
	addMana(t, e, 0, "GG2")
	opts := optionsFor(e, bears)
	if len(opts) == 0 {
		t.Fatal("sideboard creature is not offered")
	}
	submitChoices(t, e, opts[0].Index)
	fdnCensusDrain(t, e)
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Grizzly Bears did not resolve onto the battlefield: %+v", o)
	}
	if got := optionsFor(e, ring); len(got) != 0 {
		t.Errorf("second Holodeck play offered in the same turn: %+v", got)
	}
}

// Playing a land from the sideboard moves it to the battlefield, uses the
// land drop and the Holodeck's permission for the turn.
func TestHolodeckSideboardLandPlay(t *testing.T) {
	e, _, forest, ring, _, _ := holodeckEngine(t)
	addMana(t, e, 0, "2")
	var pl *decision.Option
	for _, o := range optionsFor(e, forest) {
		if o.Kind == "play_land" {
			pl = &o
		}
	}
	if pl == nil {
		t.Fatalf("sideboard land has no play_land option: %+v", optionsFor(e, forest))
	}
	submitChoices(t, e, pl.Index)
	if o := e.G.Obj(forest); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("Forest not on the battlefield: %+v", o)
	}
	if got := optionsFor(e, ring); len(got) != 0 {
		t.Errorf("second Holodeck play offered after the land: %+v", got)
	}
}

// Without a Holodeck on the battlefield the sideboard offers nothing, and it
// is not the opponent's play either.
func TestHolodeckGoneSideboardClosed(t *testing.T) {
	e, bears, forest, _, _, deck := holodeckEngine(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: deck, From: state.ZBattlefield, To: state.ZGraveyard})
	addMana(t, e, 0, "GG2")
	if len(optionsFor(e, bears))+len(optionsFor(e, forest)) != 0 {
		t.Errorf("sideboard cards offered with no Holodeck in play")
	}
}
