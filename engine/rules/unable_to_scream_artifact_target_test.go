package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 708.2, 613.1d: a face-down Splitskin Doll is a 2/2 creature with no other
// types; Unable to Scream then makes it a Toy artifact creature, so Cathartic
// Parting ("target artifact or enchantment an opponent controls") may target
// it beside the Aura. The layer-4 table must list the face-down Doll even
// though the derived Artifact Creature Toy equals the face's PRINTED words:
// the type filters read a face-down permanent's face-down words, not them.
func TestUnableToScreamMakesAFaceDownCreatureAnArtifactTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Cathartic Parting")},
		[]*cards.Card{lookup(t, reg, "Unable to Scream"), lookup(t, reg, "Splitskin Doll")})
	var doll state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 1) {
		if o := e.G.Obj(id); o.Face().Name == "Splitskin Doll" {
			doll = id
		}
	}
	if doll == 0 {
		t.Fatal("Splitskin Doll is not in seat 1's library")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: doll, From: state.ZLibrary, To: state.ZBattlefield, Counter: events.FaceDownEntryCounter})
	if o := e.G.Obj(doll); !o.FaceDown || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Doll face down = %v in %s, want a face-down battlefield permanent", o.FaceDown, o.Zone)
	}
	parting := moveByName(t, e, 0, "Cathartic Parting", state.ZHand)
	sa := e.G.Obj(parting).Face().Abilities[0]
	offers := func() []state.ObjID {
		var ids []state.ObjID
		for _, tg := range e.LegalTargets(0, parting, sa) {
			ids = append(ids, tg.Obj)
		}
		slices.Sort(ids)
		return ids
	}
	if got := offers(); len(got) != 0 {
		t.Fatalf("precondition: a face-down creature is no artifact, yet targets = %v", got)
	}
	aura := lossAttach(t, e, "Unable to Scream", 1, doll)
	if got := slices.Sorted(slices.Values(e.Derived(doll).Types)); !slices.Equal(got, []string{"Artifact", "Creature", "Toy"}) {
		t.Fatalf("precondition: Doll types = %v, want Artifact Creature Toy", got)
	}
	want := []state.ObjID{aura, doll}
	slices.Sort(want)
	if got := offers(); !slices.Equal(got, want) {
		t.Fatalf("targets = %v, want the Aura %d and the face-down Doll %d", got, aura, doll)
	}
}
