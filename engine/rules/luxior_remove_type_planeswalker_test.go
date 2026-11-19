package rules

// RemoveType$ Planeswalker (CR 205.1a) follow-through for the walker set.
// Luxior, Giada's Gift: "Equipped permanent isn't a planeswalker and is a
// creature in addition to its other types." A planeswalker's subtype is its
// own name (CR 205.3j), so stripping the Planeswalker card type must take the
// name word with it -- Forge drops "Ajani" from the type list; gorge kept the
// Planeswalker name word on the derived list (the walker vocabulary was the
// one card type the correlated-set dispatch had no word list for).
//
// The strip-side mechanic itself is pinned for the Creature set on Heliod
// (remove_type_subtype_test.go); this file pins the fifth set: the walker
// names, carried by the two corpus RemoveType$ Planeswalker statics (Luxior,
// Giada's Gift and Luxior and Shadowspear). The corpus-derived walker
// vocabulary needs the game's NameUniverse (the vocabulary is a property of
// the whole corpus, genesis.go's nameUniverseOrCorpus), so the board here
// sets it the way all_land_types_test.go does for the land vocabulary.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLuxiorRemoveTypePlaneswalkerDropsWalkerName: Ajani, Caller of the
// Pride wearing Luxior reads Legendary Creature -- the added Creature card
// type and the printed Legendary stay, the Planeswalker card type goes, and
// the walker-name subtype word "Ajani" leaves with it. The walker's own name
// is untouched: only the type-line word goes.
func TestLuxiorRemoveTypePlaneswalkerDropsWalkerName(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 47, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		NameUniverse: reg.Universe(),
		Decks: [][]*cards.Card{
			append([]*cards.Card{mustCorpusCard(t, reg, "Ajani, Caller of the Pride"),
				mustCorpusCard(t, reg, "Luxior, Giada's Gift")}, mountainDeck(t, 38)...),
			append([]*cards.Card{mustCorpusCard(t, reg, "Mountain")}, mountainDeck(t, 39)...),
		}}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	var ajani, lux state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 {
			continue
		}
		switch o.Face().Name {
		case "Ajani, Caller of the Pride":
			ajani = o.ID
		case "Luxior, Giada's Gift":
			lux = o.ID
		}
	}
	if ajani == 0 || lux == 0 {
		t.Fatalf("precondition broken: the board holds no Ajani, Caller of the Pride / Luxior, Giada's Gift pair")
	}
	for _, id := range []state.ObjID{ajani, lux} {
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
		}
	}
	e.pending = nil
	e.Advance()
	e.emit(events.Event{Kind: events.Attach, Obj: lux, IDs: []state.ObjID{ajani}})
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(ajani); o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 4 || e.G.Obj(lux).AttachedTo != ajani {
		t.Fatalf("precondition broken: Ajani in %v with %d loyalty, Luxior attached to %d", o.Zone, o.Counter("LOYALTY"), e.G.Obj(lux).AttachedTo)
	}
	if !e.IsCreature(ajani) {
		t.Fatalf("precondition broken: Luxior's Ajani is not a creature (types %v)", e.derivedTypesOf(ajani))
	}

	// Precondition: the printed line still carries the walker name -- the
	// word the strip owes is on the printed face, not only on the derived
	// list -- and the corpus-derived walker vocabulary, built from the same
	// universe the game carries, knows the word.
	face := e.G.Obj(ajani).Face()
	if got := face.Types; !hasTypeWord(got, "Ajani") || hasTypeWord(got, "Creature") {
		t.Fatalf("precondition broken: printed Ajani, Caller of the Pride types %v carry no walker-name word in the subtype slot", got)
	}
	w := chars.CorpusPlaneswalkerTypeWords(reg.Universe())
	if !slices.Contains(w, "Ajani") {
		t.Fatalf("precondition broken: the corpus walker-name vocabulary %v does not know the printed walker name", w)
	}

	ty := e.derivedTypesOf(ajani)
	if hasTypeWord(ty, "Planeswalker") {
		t.Fatalf("Luxior's Ajani still reads as a planeswalker: %v", ty)
	}
	if !hasTypeWord(ty, "Creature") || !hasTypeWord(ty, "Legendary") {
		t.Fatalf("Luxior's Ajani lost the added Creature card type or the printed Legendary: %v", ty)
	}
	if hasTypeWord(ty, "Ajani") {
		t.Fatalf("Luxior's Ajani keeps the walker-name subtype word: %v (the subtypes correlated with a removed card type leave with it, CR 205.1a; a planeswalker's subtype is its own name, CR 205.3j)", ty)
	}
	if p, tt := e.Power(ajani), e.Toughness(ajani); p != 4 || tt != 4 {
		t.Fatalf("Luxior's Ajani is %d/%d, want 4/4 (one +1/+1 per loyalty counter)", p, tt)
	}
}
