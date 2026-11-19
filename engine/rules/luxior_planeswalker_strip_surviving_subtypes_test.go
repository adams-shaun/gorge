package rules

// CR 205.1a's survival clause for the RemoveType$ Planeswalker follow-through:
// a subtype correlated with the removed card type leaves with it "unless it is
// also a subtype of a card type the object still has". The walker-name
// vocabulary is derived from the words printed in the subtype slot of every
// Planeswalker face, and the corpus prints two kinds of collision there:
//
//   - The Aetherspark is "Legendary Artifact Planeswalker Equipment": the word
//     Equipment is its walker-name slot AND an artifact subtype, and the face
//     carries the Artifact card type. Luxior's strip names Planeswalker only,
//     so the object still has the Artifact card type and the word stays.
//   - Drake Stone is "Legendary Planeswalker Stone": "Stone" is also an
//     artifact subtype word, but the face carries no Artifact card type, so
//     the word leaves with the Planeswalker type.
//   - Dungeon Master is "Legendary Planeswalker Dungeon Master": both halves
//     of the name are subtype-slot words ("Dungeon" is a card-type WORD, but
//     not on this face), and both must leave -- otherwise the stripped walker
//     reads as a Legendary Dungeon.
//
// The corpus-derived walker vocabulary needs the game's NameUniverse the way
// luxior_remove_type_planeswalker_test.go sets it up.
import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLuxiorPlaneswalkerStripKeepsSubtypesOfRemainingTypes: Luxior, Giada's
// Gift on each walker face whose subtype-slot word overlaps another
// vocabulary. The strip takes the words whose card type it removed and keeps
// the words a remaining card type still claims (CR 205.1a).
func TestLuxiorPlaneswalkerStripKeepsSubtypesOfRemainingTypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		walker string
		want   []string // type words the stripped permanent must still carry
		absent []string // type words that must have left
	}{
		{"The Aetherspark",
			[]string{"Legendary", "Artifact", "Creature", "Equipment"},
			[]string{"Planeswalker"}},
		{"Drake Stone",
			[]string{"Legendary", "Creature"},
			[]string{"Planeswalker", "Stone"}},
		{"Dungeon Master",
			[]string{"Legendary", "Creature"},
			[]string{"Planeswalker", "Dungeon", "Master"}},
	}
	for _, tc := range cases {
		t.Run(tc.walker, func(t *testing.T) {
			t.Parallel()
			// Precondition: the printed face carries every word the case
			// speaks about, in its own subtype slot, under the Planeswalker
			// card type.
			face := mustCorpusCard(t, reg, tc.walker).Faces[0]
			if !slices.Contains(face.Types, "Planeswalker") {
				t.Fatalf("precondition broken: %s prints no Planeswalker card type: %v", tc.walker, face.Types)
			}
			for _, w := range append(append([]string{}, tc.want...), tc.absent...) {
				if w == "Creature" {
					continue // the added type, not a printed word
				}
				if !hasTypeWord(face.Types, w) {
					t.Fatalf("precondition broken: %s's printed types %v lack %q", tc.walker, face.Types, w)
				}
			}
			w := chars.CorpusPlaneswalkerTypeWords(reg.Universe())
			for _, word := range tc.absent {
				if word == "Planeswalker" {
					continue // the card type, not a vocabulary word
				}
				if !slices.Contains(w, word) {
					t.Fatalf("precondition broken: the corpus walker-name vocabulary %v does not know %q", w, word)
				}
			}

			cfg := Config{Seed: 47, Names: []string{"a", "b"}, Tokens: reg.Tokens,
				NameUniverse: reg.Universe(),
				Decks: [][]*cards.Card{
					append([]*cards.Card{mustCorpusCard(t, reg, tc.walker),
						mustCorpusCard(t, reg, "Luxior, Giada's Gift")}, mountainDeck(t, 38)...),
					append([]*cards.Card{mustCorpusCard(t, reg, "Mountain")}, mountainDeck(t, 39)...),
				}}
			cfg = seatZeroStart(cfg)
			e := New(cfg)
			var walker, lux state.ObjID
			for i := range e.G.Objs {
				o := &e.G.Objs[i]
				if o.Owner != 0 {
					continue
				}
				switch o.Face().Name {
				case tc.walker:
					walker = o.ID
				case "Luxior, Giada's Gift":
					lux = o.ID
				}
			}
			if walker == 0 || lux == 0 {
				t.Fatalf("precondition broken: the board holds no %s / Luxior, Giada's Gift pair", tc.walker)
			}
			for _, id := range []state.ObjID{walker, lux} {
				o := e.G.Obj(id)
				if o.Zone != state.ZBattlefield {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
				}
			}
			e.pending = nil
			e.Advance()
			ty := e.derivedTypesOf(walker)
			if !hasTypeWord(ty, "Planeswalker") {
				t.Fatalf("precondition broken: %s entered the battlefield without the Planeswalker card type: %v", tc.walker, ty)
			}
			e.emit(events.Event{Kind: events.Attach, Obj: lux, IDs: []state.ObjID{walker}})
			e.pending = nil
			e.Advance()
			if e.G.Obj(lux).AttachedTo != walker {
				t.Fatalf("precondition broken: Luxior is not attached to %s", tc.walker)
			}

			ty = e.derivedTypesOf(walker)
			for _, w := range tc.want {
				if !hasTypeWord(ty, w) {
					t.Fatalf("Luxior's %s lost type word %q: %v (a subtype correlated with the removed Planeswalker type stays when it is also the subtype of a card type the object still has, CR 205.1a)", tc.walker, w, ty)
				}
			}
			for _, w := range tc.absent {
				if hasTypeWord(ty, w) {
					t.Fatalf("Luxior's %s keeps type word %q: %v (the subtypes correlated with a removed card type leave with it, CR 205.1a)", tc.walker, w, ty)
				}
			}
		})
	}
}

// TestWalkerNameVocabularyMatchesPlaneswalkerTypeChoices: the corpus-derived
// walker-name vocabulary (the strip side, rules/chars) and the Type$
// Planeswalker choice list (engine/effects choosePlaneswalkerTypes) are two
// readings of the same corpus fact -- every word printed in the subtype slot
// of a Planeswalker face -- and must agree, so the next reader of either list
// inherits one vocabulary, not two.
func TestWalkerNameVocabularyMatchesPlaneswalkerTypeChoices(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	derived := chars.CorpusPlaneswalkerTypeWords(reg.Universe())
	choices := effects.TypeChoiceLabels("Planeswalker", "", "")
	if len(derived) == 0 || len(choices) == 0 {
		t.Fatalf("precondition broken: derived walker vocabulary %v / choice labels %v empty", derived, choices)
	}
	if !slices.Equal(derived, choices) {
		t.Fatalf("the derived walker-name vocabulary and the Type$ Planeswalker choice list disagree:\nderived: %v\nchoices: %v", derived, choices)
	}
}
