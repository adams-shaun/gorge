package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestVerdantCrescendoSearchesItsCompoundOrigin is driven by Verdant
// Crescendo's real DBSearch: Origin$ Library,Graveyard with no object
// selector. The search decision must contain the union, not fall through to
// Defined's source default. The graveyard answer is then resumed explicitly,
// proving that the chosen object (rather than the first library card) moves.
func TestVerdantCrescendoSearchesItsCompoundOrigin(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Verdant Crescendo")
	if !ok {
		t.Skip("corpus missing Verdant Crescendo")
	}
	var search *cards.SA
	for _, ability := range card.Faces[0].Abilities {
		if ability.Sub != nil && ability.Sub.Params["Origin"] == "Library,Graveyard" {
			search = ability.Sub
			break
		}
	}
	if search == nil {
		t.Fatal("corpus pin moved: Verdant Crescendo has no Library,Graveyard DBSearch")
	}

	h := &askHost{}
	h.g = state.NewGame(names(2))
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	libraryCard := h.g.AddObject(mkCard(t, "Name:Nissa, Nature's Artisan\nTypes:Planeswalker\nOracle:x\n"), 0)
	libraryCard.Zone = state.ZLibrary
	graveyardCard := h.g.AddObject(mkCard(t, "Name:Nissa, Nature's Artisan\nTypes:Planeswalker\nOracle:x\n"), 0)
	graveyardCard.Zone = state.ZGraveyard
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{libraryCard.ID})
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{graveyardCard.ID})
	if libraryCard.Zone == graveyardCard.Zone || libraryCard.ID == graveyardCard.ID {
		t.Fatal("test setup did not create distinct library and graveyard candidates")
	}

	h.asked = nil
	ctx := &Ctx{Source: source.ID, Controller: 0}
	effChangeZone(h, ctx, search)
	if h.asked == nil {
		t.Fatal("compound-origin search did not pose a chooser")
	}
	if h.asked.Kind != decision.KChoose || h.asked.ResumeKind != "search" {
		t.Fatalf("decision = %+v, want a search KChoose", h.asked)
	}
	contains := func(id state.ObjID) bool {
		for _, option := range h.asked.Options {
			if option.Obj == id {
				return true
			}
		}
		return false
	}
	if !contains(libraryCard.ID) || !contains(graveyardCard.ID) {
		t.Fatalf("options = %+v, want both compound-origin candidates", h.asked.Options)
	}

	ctx.Search = []state.ObjID{graveyardCard.ID}
	ctx.SearchDone = true
	ctx.LibraryTarget = 0
	effChangeZone(h, ctx, search)
	if got := h.g.Obj(graveyardCard.ID).Zone; got != state.ZHand {
		t.Fatalf("answered graveyard candidate zone = %v, want hand", got)
	}
	if got := h.g.Obj(libraryCard.ID).Zone; got != state.ZLibrary {
		t.Fatalf("unchosen library candidate zone = %v, want library", got)
	}
}

// TestCompoundLibraryAndHandOriginUsesOneUnionChooser covers the mixed hidden
// origin that previously emitted the loud fallback note. Both candidates are
// in the controller's named origins, so a source-default object path would be
// observably wrong.
func TestCompoundLibraryAndHandOriginUsesOneUnionChooser(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	source := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0)
	c := &Ctx{Source: source.ID, Controller: 0}
	libraryCard := h.g.AddObject(mkCard(t, "Name:Library Creature\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	libraryCard.Zone = state.ZLibrary
	handCard := h.g.AddObject(mkCard(t, "Name:Hand Creature\nTypes:Creature\nPT:3/3\nOracle:x\n"), 0)
	handCard.Zone = state.ZHand
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{libraryCard.ID})
	h.g.SetZone(state.ZHand, 0, []state.ObjID{handCard.ID})
	if libraryCard.Zone == handCard.Zone || libraryCard.ID == handCard.ID {
		t.Fatal("test setup did not create distinct library and hand candidates")
	}

	s := sa(t, "DB$ ChangeZone | Origin$ Library,Hand | Destination$ Battlefield | ChangeType$ Creature")
	h.asked = nil
	effChangeZone(h, c, s)
	if h.asked == nil {
		t.Fatal("mixed library/hand origin did not pose a chooser")
	}
	if h.asked.ResumeKind != "search" || len(h.asked.Options) != 2 {
		t.Fatalf("decision = %+v, want one search decision with two options", h.asked)
	}
	for _, id := range []state.ObjID{libraryCard.ID, handCard.ID} {
		found := false
		for _, option := range h.asked.Options {
			if option.Obj == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("candidate %d absent from union options: %+v", id, h.asked.Options)
		}
	}
}
