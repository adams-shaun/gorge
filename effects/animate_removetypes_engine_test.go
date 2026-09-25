package effects_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// An inline Weeping Angel trigger body: no Forge script is included in the fixture.
func TestWeepingAngelRemoveTypesChangesDerivedCharacteristics(t *testing.T) {
	const src = "Name:Weeping Angel\nManaCost:1 U B\nTypes:Artifact Creature Alien Angel\nPT:2/2\n" +
		"T:Mode$ SpellCast | ValidCard$ Creature | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ TrigAnimate\n" +
		"SVar:TrigAnimate:DB$ Animate | RemoveTypes$ Creature\nOracle:x\n"
	card, diags := cards.ParseBytes("inline-angel.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	card.Link()
	body := cards.ResolveSVar(card.Faces[0].SVars, "TrigAnimate")
	if len(card.Faces[0].Triggers) == 0 || body == nil || body.API != "Animate" || body.Params["RemoveTypes"] != "Creature" {
		t.Fatalf("precondition: missing battlefield spell-cast Animate trigger: %+v", body)
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = card
	}
	e := rules.New(rules.Config{Seed: 42, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}})
	id := e.G.Zone(state.ZLibrary, 0)[0]
	o := e.G.Obj(id)
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield, Player: 0})
	if e.G.Obj(id).Zone != state.ZBattlefield || !slices.Contains(e.Derived(id).Types, "Creature") || !e.IsCreature(id) {
		t.Fatalf("precondition: printed Angel must be a battlefield creature: %+v", e.Derived(id))
	}
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, body)
	d := e.Derived(id)
	if !slices.Equal(d.Types, []string{"Artifact", "Alien", "Angel"}) || e.IsCreature(id) {
		t.Fatalf("animated Angel types = %v creature = %v; want Artifact Alien Angel noncreature", d.Types, e.IsCreature(id))
	}
}

func TestAnimateNamedRemoveTypesAcrossCategories(t *testing.T) {
	const src = "Name:Test Plate\nTypes:Snow Artifact Equipment\n" +
		"SVar:AnimateIt:DB$ Animate | RemoveTypes$ Snow, Equipment | Types$ Creature, Spirit\nOracle:x\n"
	card, diags := cards.ParseBytes("inline-plate.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	card.Link()
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = card
	}
	e := rules.New(rules.Config{Seed: 43, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}})
	id := e.G.Zone(state.ZLibrary, 0)[0]
	o := e.G.Obj(id)
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield, Player: 0})
	before := e.Derived(id).Types
	if e.G.Obj(id).Zone != state.ZBattlefield || !slices.Contains(before, "Snow") || !slices.Contains(before, "Equipment") || slices.Contains(before, "Spirit") {
		t.Fatalf("precondition: printed plate types = %v, zone = %v", before, e.G.Obj(id).Zone)
	}
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0}, cards.ResolveSVar(card.Faces[0].SVars, "AnimateIt"))
	if got := e.Derived(id).Types; !slices.Equal(got, []string{"Artifact", "Creature", "Spirit"}) {
		t.Fatalf("animated plate types = %v, want [Artifact Creature Spirit]", got)
	}
}
