package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// findCorpusAbility walks one face's abilities and trigger effects (and their
// SubAbility$ chains) for the ChangeZone SA matching pred.
func findCorpusAbility(face *cards.Face, pred func(*cards.SA) bool) *cards.SA {
	var walk func(*cards.SA) *cards.SA
	walk = func(s *cards.SA) *cards.SA {
		if s == nil {
			return nil
		}
		if pred(s) {
			return s
		}
		return walk(s.Sub)
	}
	for _, ab := range face.Abilities {
		if got := walk(ab); got != nil {
			return got
		}
	}
	for i := range face.Triggers {
		if got := walk(face.Triggers[i].Effect); got != nil {
			return got
		}
	}
	return nil
}

// TestHandMoveUpToIsOptional pins the corpus Gitrog, Ravenous Ride shape: its
// "put UP TO X land cards from your hand onto the battlefield" is a may-pick
// even though the sub-ability carries neither Mandatory$ nor Optional$. With
// one land in hand and X = 1 the only possible nonempty answer takes the
// land, but declining is still legal, so the walk must pose Min 0 / Max 1 and
// not fold the move into the take-all branch.
func TestHandMoveUpToIsOptional(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("The Gitrog, Ravenous Ride")
	if !ok {
		t.Fatal("corpus has no The Gitrog, Ravenous Ride")
	}
	ab := findCorpusAbility(card.Faces[0], func(s *cards.SA) bool {
		return s.API == "ChangeZone" && s.Params["Origin"] == "Hand" &&
			s.Params["ChangeNum"] == "Sacrificed$CardPower"
	})
	if ab == nil {
		t.Fatalf("Gitrog has no hand ChangeZone with ChangeNum$ Sacrificed$CardPower: %+v", card.Faces[0].Abilities)
	}
	if ab.Params["Mandatory"] != "" || ab.Params["Optional"] != "" {
		t.Fatalf("Gitrog's change-zone marker drifted: %+v", ab.Params)
	}
	if !strings.Contains(strings.ToLower(card.Faces[0].Oracle), "put up to") {
		t.Fatalf("Gitrog's oracle no longer says 'put up to': %q", card.Faces[0].Oracle)
	}

	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "The Gitrog, Ravenous Ride")
	land := corpusObject(t, reg, g, "Forest")
	land.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{land.ID})
	h := &askHost{}
	h.g = g

	Resolve(h, &Ctx{Controller: 0, Source: source.ID,
		Sacrificed: []state.SacrificedInfo{{Obj: source.ID, Power: 1}}}, ab)
	if h.asked == nil {
		t.Fatal("no decision was posed: 'put up to X' must offer the zero-card answer")
	}
	d := h.asked
	if d.Player != 0 || d.Min != 0 || d.Max != 1 {
		t.Fatalf("asked Player %d Min/Max %d/%d, want Player 0 Min 0 Max 1", d.Player, d.Min, d.Max)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != land.ID {
		t.Fatalf("options = %+v, want exactly the one land in hand", d.Options)
	}
}

// TestHandMoveForcedLibraryBottomAsksPlacementOrder pins CR 401.4 on the
// corpus Sawtooth Loon shape: its required "put two cards from your hand on
// the bottom of your library" has no selection alternative with two cards in
// hand, so the take-all branch moves both, but their owner must still arrange
// them -- one library_placement KArrange over both.
func TestHandMoveForcedLibraryBottomAsksPlacementOrder(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Sawtooth Loon")
	if !ok {
		t.Fatal("corpus has no Sawtooth Loon")
	}
	ab := findCorpusAbility(card.Faces[0], func(s *cards.SA) bool {
		return s.API == "ChangeZone" && s.Params["Origin"] == "Hand" &&
			s.Params["Destination"] == "Library" && s.Params["ChangeNum"] == "2" &&
			s.Params["Mandatory"] == "True" && s.Params["LibraryPosition"] == "-1"
	})
	if ab == nil {
		t.Fatalf("Sawtooth Loon has no hand-to-library ChangeZone: %+v", card.Faces[0].Abilities)
	}
	if !strings.Contains(strings.ToLower(card.Faces[0].Oracle), "put two cards from your hand on the bottom of your library") {
		t.Fatalf("Sawtooth Loon's oracle drifted: %q", card.Faces[0].Oracle)
	}

	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "Sawtooth Loon")
	island := corpusObject(t, reg, g, "Island")
	plains := corpusObject(t, reg, g, "Plains")
	for _, o := range []*state.Object{island, plains} {
		o.Zone = state.ZHand
	}
	g.SetZone(state.ZHand, 0, []state.ObjID{island.ID, plains.ID})
	if got := g.Zone(state.ZHand, 0); len(got) != 2 {
		t.Fatalf("hand = %v, want the two cards the take-all moves", got)
	}
	h := &askHost{}
	h.g = g

	Resolve(h, &Ctx{Controller: 0, Source: source.ID}, ab)
	if h.asked == nil {
		t.Fatal("no decision was posed: two cards put into one library position need CR 401.4 arrangement")
	}
	d := h.asked
	if d.Kind != decision.KArrange || d.Player != 0 || d.Min != 2 || d.Max != 2 {
		t.Fatalf("asked Kind %v Player %d Min/Max %d/%d, want KArrange Player 0 Min 2 Max 2",
			d.Kind, d.Player, d.Min, d.Max)
	}
	if len(d.Options) != 2 {
		t.Fatalf("options = %d, want both moved cards", len(d.Options))
	}
	for _, o := range d.Options {
		if o.Kind != "library_placement" || (o.Obj != island.ID && o.Obj != plains.ID) {
			t.Fatalf("option %+v is not a library_placement of one of the moved cards", o)
		}
	}
}
