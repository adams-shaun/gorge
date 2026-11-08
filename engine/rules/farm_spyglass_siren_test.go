package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// farmSpyglassSirenLibrary is the p1.library_top the farm reported for
// LCI-s274's window xe:RAND:2:15:w14 at step 33 (the LCI-s339 row has the same
// shape). It is exactly the order Hurl into History's Discover 1 exiled the
// cards in: none of the five qualifies (lands, or nonlands with mana value
// above 1), so every one is part of CR 701.57a's "remaining exiled cards",
// which go to the bottom "in a random order".
var farmSpyglassSirenLibrary = []string{"Island", "Waterwind Scout", "Panicked Altisaur", "Island", "Island"}

// farmSpyglassSirenCard builds one inline fixture face for the observed
// library. The fixture texts are ours, not the Forge scripts (never committed).
func farmSpyglassSirenCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	switch name {
	case "Island":
		return card(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
	case "Waterwind Scout":
		return card(t, "Name:Waterwind Scout\nManaCost:2 U\nTypes:Creature Merfolk Scout\nPT:2/1\nOracle:x\n")
	case "Panicked Altisaur":
		return card(t, "Name:Panicked Altisaur\nManaCost:4 R R\nTypes:Creature Dinosaur\nPT:5/5\nOracle:x\n")
	}
	t.Fatalf("no inline fixture for %q", name)
	return nil
}

// discoverFarmLibrary stages the observed library in front of a Discover 1
// (the X Hurl into History gets from a countered Spyglass Siren's mana value)
// and returns the resulting library order plus the number of rng values the
// resolution drew.
func discoverFarmLibrary(t *testing.T, seed uint64) (*Engine, []state.ObjID, uint64) {
	t.Helper()
	source := card(t, "Name:Hurl into History\nManaCost:3 U U\nTypes:Instant\nOracle:x\n")
	lib := make([]*cards.Card, len(farmSpyglassSirenLibrary))
	for i, name := range farmSpyglassSirenLibrary {
		lib[i] = farmSpyglassSirenCard(t, name)
	}
	cfg := Config{Seed: seed, Names: []string{"a", "b"}, Tokens: map[string]*cards.Card{}}
	st := Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1,
		Players:    []StagedPlayer{{Life: 20, Library: lib}, {Life: 20}},
		Permanents: []StagedPermanent{{Card: source, Owner: 0, Controller: 0}},
	}
	e, ids, err := NewStaged(cfg, st)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if len(ids.Library[0]) != len(farmSpyglassSirenLibrary) {
		t.Fatalf("staged library %d cards, want %d", len(ids.Library[0]), len(farmSpyglassSirenLibrary))
	}
	// Precondition: the whole library is the "rest" pile, so the test's
	// assertion is about the bottom order and not about the scan stopping.
	for i, id := range ids.Library[0] {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			t.Fatalf("staged library card %d has no face", i)
		}
		if !o.Face().IsLand() && o.Face().ManaValue() <= 1 {
			t.Fatalf("precondition: library card %d (%s) qualifies for Discover 1", i, o.Face().Name)
		}
	}
	before := e.RNGDraws()
	c := effects.NewCtx(ids.Permanents[0], 0, effects.CtxInit{})
	effects.Resolve(e, &c, &cards.SA{Kind: "DB", API: "Discover", Params: map[string]string{"Num": "1"}})
	return e, append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...), e.RNGDraws() - before
}

func libraryNames(e *Engine, ids []state.ObjID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = e.G.Obj(id).Face().Name
	}
	return out
}

// TestFarmSpyglassSiren is the farm regression pin. The two farm rows
// (LCI-s274 window xe:RAND:2:15:w14, LCI-s339 window xe:RAND:3:12:w15) are
// 3way:G|F|X on p1.library_top: Forge and XMage each randomise Discover's
// bottom pile, so they differ from each other AND from gorge, which left the
// cards in scan order. The first differing value was the observed gorge list
// below; the fix is CR 701.57a's random order, drawn from the engine's seeded
// rng.
func TestFarmSpyglassSiren(t *testing.T) {
	t.Parallel()
	eA, libA, drawsA := discoverFarmLibrary(t, 274)
	eB, libB, drawsB := discoverFarmLibrary(t, 339)

	// Precondition: both engines still hold exactly the observed cards, so a
	// difference below is an order difference, not a lost/extra card.
	if got := libraryNames(eA, libA); len(got) != len(farmSpyglassSirenLibrary) {
		t.Fatalf("precondition: library = %v, want the observed %v", got, farmSpyglassSirenLibrary)
	}
	if got := libraryNames(eB, libB); len(got) != len(farmSpyglassSirenLibrary) {
		t.Fatalf("precondition: library = %v, want the observed %v", got, farmSpyglassSirenLibrary)
	}

	if got := libraryNames(eA, libA); strings.Join(got, ",") == strings.Join(farmSpyglassSirenLibrary, ",") {
		t.Fatalf("p1.library_top = %v: Discover bottomed its rest pile in scan order. "+
			"Farm LCI-s274 (xe:RAND:2:15:w14, class 3way:G|F|X) read this exact list while "+
			"Forge and XMage each put the same cards on the bottom in their own random order; "+
			"CR 701.57a requires the random order.", got)
	}
	if drawsA != uint64(len(farmSpyglassSirenLibrary)-1) {
		t.Fatalf("Discover drew %d rng values, want %d (one Fisher-Yates pass over the %d-card rest pile)",
			drawsA, len(farmSpyglassSirenLibrary)-1, len(farmSpyglassSirenLibrary))
	}
	if drawsB != drawsA {
		t.Fatalf("the same Discover drew %d and %d rng values under two seeds", drawsA, drawsB)
	}
	// Two seeds must produce different permutations: a fixed rearrangement
	// would still fail CR 701.57a.
	if strings.Join(libraryNames(eA, libA), ",") == strings.Join(libraryNames(eB, libB), ",") {
		t.Fatalf("seeds 274 and 339 produced the same bottom order %v; the order is not random",
			libraryNames(eA, libA))
	}
}
