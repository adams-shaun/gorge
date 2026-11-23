package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 113.7a / 608.2h: a source that left the battlefield is read as it last
// existed. An animated Celestial Colonnade (white and blue) that dies is a
// colourless land card in the graveyard, yet "Card.Blue" must still match it
// as an ability source until it changes zones again.
func TestDepartedSourceKeepsLastKnownColours(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Celestial Colonnade")}, []*cards.Card{})
	id := moveByName(t, e, 0, "Celestial Colonnade", state.ZBattlefield)
	addMana(t, e, 0, "WWWWUU")
	submitChoices(t, e, animateAbilityOption(t, e, id).Index)
	settleActivation(t, e)
	if got := e.Colors(id); got != "WU" {
		t.Fatalf("precondition: animated colours = %q, want WU", got)
	}

	blue := func() bool {
		return func() bool { m, ok := e.matchesDepartedSource("Card.Blue", id, e.specCtx(0, 1)); return ok && m }()
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard || e.Colors(id) != "" {
		t.Fatalf("precondition: zone %v colours %q, want a colourless graveyard card", o.Zone, e.Colors(id))
	}
	if e.matchesSpec("Card.Blue", id, e.specCtx(0, 1)) {
		t.Fatalf("precondition: the plain graveyard read already matches Card.Blue")
	}
	if !blue() {
		t.Fatalf("departed animated land does not match Card.Blue through its last-known colours")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile})
	if blue() {
		t.Fatalf("a later zone change must retire the departure colours")
	}
}
