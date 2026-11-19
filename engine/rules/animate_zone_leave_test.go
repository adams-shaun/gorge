package rules_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7: an object that leaves the battlefield is a new object, and
// effects from spells, activated abilities, and triggered abilities that
// change its characteristics are not among the 400.7a-i continuations. An
// until-end-of-turn Animate therefore ends when the animated object leaves
// the battlefield even without a rider: a land animated by Genju of the
// Fields, put on top of its library (Submerge) and replayed the same turn is
// a plain land, not a re-animated 2/5 Spirit.
func TestAnimateEndsWhenObjectLeavesBattlefield(t *testing.T) {
	const land = "Name:Test Plains\nTypes:Land\n" +
		"SVar:A:AB$ Animate | Cost$ 2 | Defined$ Self | Power$ 2 | Toughness$ 5 | Types$ Creature,Spirit | Colors$ White\n" +
		"Oracle:x\n"
	card, diags := cards.ParseBytes("inline-plains.txt", []byte(land))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	card.Link()
	body := cards.ResolveSVar(card.Faces[0].SVars, "A")
	if body == nil || body.API != "Animate" {
		t.Fatalf("precondition: missing Animate body: %+v", body)
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = card
	}
	e := rules.New(rules.Config{Seed: 42, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}})
	id := e.G.Zone(state.ZLibrary, 0)[0]
	o := e.G.Obj(id)
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield, Player: 0})
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, body)
	d := e.Derived(id)
	if d.Power != 2 || !e.IsCreature(id) {
		t.Fatalf("precondition: animated land must be a 2/5 creature (power %d, creature %v)", d.Power, e.IsCreature(id))
	}
	// Put it on top of its owner's library and play it again the same turn.
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZLibrary, Player: 0})
	o = e.G.Obj(id)
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield, Player: 0})
	d = e.Derived(id)
	if d.Power != 0 || e.IsCreature(id) {
		t.Fatalf("replayed land power %d creature %v types %v; want a plain land (CR 400.7)", d.Power, e.IsCreature(id), d.Types)
	}
}

// The pool's own shape: the Genju ability was activated while the aura was
// attached, then the Submerge put the animated Plains on top of its owner's
// library and the aura fell off to the graveyard, and only THEN did the
// Animate ability resolve. The grant is made to a card in the library; the
// draw that follows is a zone move (CR 400.7: the drawn card is a new
// object), so the grant ends with it and the later replay of that card is a
// plain permanent, never a revived animation.
func TestAnimateGrantInLibraryEndsWithTheDraw(t *testing.T) {
	const beast = "Name:Test Beast\nTypes:Creature Golem\nPT:3/3\nOracle:x\n"
	const aura = "Name:Test Genju\nManaCost:W\nTypes:Enchantment Aura\n" +
		"SVar:A:AB$ Animate | Cost$ 2 | Defined$ Enchanted | Power$ 2 | Toughness$ 5 | Types$ Creature,Spirit | Colors$ White\n" +
		"Oracle:x\n"
	beastCard, diags := cards.ParseBytes("inline-beast.txt", []byte(beast))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	auraCard, diags := cards.ParseBytes("inline-genju.txt", []byte(aura))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	auraCard.Link()
	body := cards.ResolveSVar(auraCard.Faces[0].SVars, "A")
	if body == nil || body.API != "Animate" {
		t.Fatalf("precondition: missing Animate body: %+v", body)
	}
	deck0 := make([]*cards.Card, 40)
	for i := range deck0 {
		deck0[i] = beastCard
	}
	deck0[9] = auraCard
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = beastCard
	}
	e := rules.New(rules.Config{Seed: 42, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck0, deck1}})
	var bID, aID state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		switch e.Derived(id).Name {
		case "Test Beast":
			if bID == 0 {
				bID = id
			}
		case "Test Genju":
			if aID == 0 {
				aID = id
			}
		}
	}
	if bID == 0 || aID == 0 {
		t.Fatalf("precondition: both cards must be in the library (beast %d, aura %d)", bID, aID)
	}
	// Attach and resolve with both cards off the battlefield, the pool's own
	// order: the aura fell off before its ability resolved, so the bearer is
	// read from the attachment the aura left behind.
	e.Emit(events.Event{Kind: events.Attach, Obj: aID, IDs: []state.ObjID{bID}, Player: 0})
	if e.G.Obj(aID).AttachedTo != bID {
		t.Fatalf("precondition: aura not attached to the beast: %+v", e.G.Obj(aID))
	}
	effects.Resolve(e, &effects.Ctx{Source: aID, Controller: 0}, body)
	if d := e.Derived(bID); d.Power != 2 {
		t.Fatalf("precondition: the grant must apply to the card it was made for (power %d)", d.Power)
	}
	// The draw: the same event shape resumeOrdinaryDraw emits.
	e.Emit(events.Event{Kind: events.Draw, Player: 0, Obj: bID, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if e.G.Obj(bID).Zone != state.ZHand {
		t.Fatalf("precondition: the beast did not move to hand (zone %d)", e.G.Obj(bID).Zone)
	}
	if d := e.Derived(bID); d.Power != 3 || slices.Contains(d.Types, "Spirit") || d.Colors != "" {
		t.Fatalf("drawn card power %d types %v colors %v; the grant must end when the card leaves the library (CR 400.7)", d.Power, d.Types, d.Colors)
	}
	// And the later replay of the same card must be a plain permanent.
	e.Emit(events.Event{Kind: events.MoveZone, Obj: bID, From: state.ZHand, To: state.ZBattlefield, Player: 0})
	if d := e.Derived(bID); d.Power != 3 || slices.Contains(d.Types, "Spirit") || d.Colors != "" {
		t.Fatalf("replayed creature power %d types %v colors %v; want the printed permanent (CR 400.7)", d.Power, d.Types, d.Colors)
	}
}
