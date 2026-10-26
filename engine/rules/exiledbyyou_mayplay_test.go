package rules

// exiledbyyou_mayplay_test.go pins Card.ExiledByYou on the two corpus cards that
// read it (Evelyn, the Covetous; Haldan, Avid Arcanist): a card the controller's
// own Dig exiled, with the cr-g190 counter, is offered to be played from exile.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExiledByYouEvelynCollectionOffer: Evelyn's ETB Dig exiles the top of every
// library with a COLLECTION counter; her static then lets her controller play
// the lands among them (Card.ExiledByYou+counters_GE1_COLLECTION).
func TestExiledByYouEvelynCollectionOffer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{Setup: map[string]oracleSeat{
		"p0": {Battlefield: []string{"Evelyn, the Covetous"}, LibraryTop: []string{"Mountain"}},
		"p1": {LibraryTop: []string{"Grizzly Bears"}},
	}}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	mountain := digExileNamed(t, e, "Mountain")
	bears := digExileNamed(t, e, "Grizzly Bears")
	for _, o := range []*state.Object{mountain, bears} {
		if o.Counter("COLLECTION") != 1 {
			t.Fatalf("precondition: %s COLLECTION = %d, want 1", o.Face().Name, o.Counter("COLLECTION"))
		}
		if !o.HasExiledBy || o.ExiledBy != 0 {
			t.Fatalf("precondition: %s exiler = (%v, %d), want (true, 0)", o.Face().Name, o.HasExiledBy, o.ExiledBy)
		}
	}
	offered := false
	for _, id := range e.mayPlayLandIds(0) {
		if id == mountain.ID {
			offered = true
		}
	}
	if !offered {
		t.Errorf("mayPlayLandIds(0) = %v, want it to contain the exiled Mountain %d", e.mayPlayLandIds(0), mountain.ID)
	}
	if d := gameStateDiff(run.e.G, logReplayGame(run.cfg, run.e.L.Events)); d != "" {
		t.Errorf("log-only replay differs:\n%s", d)
	}
}

// TestExiledByYouHaldanFetchOffer: Pako's attack trigger exiles the top of every
// library with a FETCH counter; Haldan's static lets its controller play the
// noncreature ones (Card.nonCreature+ExiledByYou+counters_GE1_FETCH).
func TestExiledByYouHaldanFetchOffer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{Setup: map[string]oracleSeat{
		"p0": {Battlefield: []string{"Pako, Arcane Retriever", "Haldan, Avid Arcanist"}},
		"p1": {LibraryTop: []string{"Lotus Petal"}},
	}, Steps: []oracleStep{
		{Op: "attack", Seat: 0, Attackers: []string{"p0:Pako, Arcane Retriever"}, Defender: "p1"},
		{Op: "resolve"},
	}}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	petal := digExileNamed(t, e, "Lotus Petal")
	if petal.Counter("FETCH") != 1 {
		t.Fatalf("precondition: Lotus Petal FETCH = %d, want 1", petal.Counter("FETCH"))
	}
	if !petal.HasExiledBy || petal.ExiledBy != 0 {
		t.Fatalf("precondition: Lotus Petal exiler = (%v, %d), want (true, 0)", petal.HasExiledBy, petal.ExiledBy)
	}
	if petal.Face().IsCreature() {
		t.Fatalf("precondition: Lotus Petal must be a noncreature")
	}
	offered := false
	for _, of := range e.mayPlaySpellIds(0) {
		if of.id == petal.ID {
			offered = true
		}
	}
	if !offered {
		t.Errorf("mayPlaySpellIds(0) = %+v, want an offer for the exiled Lotus Petal %d", e.mayPlaySpellIds(0), petal.ID)
	}
	if d := gameStateDiff(run.e.G, logReplayGame(run.cfg, run.e.L.Events)); d != "" {
		t.Errorf("log-only replay differs:\n%s", d)
	}
}
