package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.24d: "If an effect would cause a player to shuffle a set of objects
// into a library, that library is shuffled even if there are no objects in
// that set." These farm scenarios pin the object-target ChangeZone tail's
// named-library shuffle when the target ask answered zero cards: Renewing
// Touch's "target creature you control" (your library), Dwell on the Past's
// "target player" (the parent player's library), Turn the Earth's "those
// cards" (owner-defined: no library is named when no card is chosen), and
// the one-target Renewing Touch control.

func emptySetShuffleSnapshot(t *testing.T, res OracleResult, checkpoint string) OracleSnapshot {
	t.Helper()
	for i := range res.Snapshots {
		if res.Snapshots[i].Checkpoint == checkpoint {
			return res.Snapshots[i]
		}
	}
	t.Fatalf("no %q snapshot", checkpoint)
	return OracleSnapshot{}
}

func TestFarmRenewingTouch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm renewing touch zero targets","turn":1,"cr":["701.24d"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Forest"],"hand":["Renewing Touch"],"graveyard":["Grizzly Bears"],` +
		`"library_top":["Forest","Plains","Plains","Swamp","Island"],"life":20},` +
		`"p1":{"library_top":["Plains","Island","Swamp","Mountain","Forest"],"life":20}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Renewing Touch","mana":"G"},` +
		`{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	before := emptySetShuffleSnapshot(t, res, "step 0 (cast)")
	if !slices.Contains(before.Players[0].Graveyard, "Grizzly Bears") {
		t.Fatalf("precondition: p0 graveyard after cast = %v, want the zero-target ask's Grizzly Bears still there", before.Players[0].Graveyard)
	}
	after := emptySetShuffleSnapshot(t, res, "step 1 (resolve)")
	setup := []string{"Forest", "Plains", "Plains", "Swamp", "Island"}
	if slices.Equal(after.Players[0].LibraryTop, setup) {
		t.Fatalf("p0 library top after a zero-target Renewing Touch = %v, want a shuffled library (CR 701.24d)", after.Players[0].LibraryTop)
	}
}

func TestFarmRenewingTouchTargets(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm renewing touch one target","turn":1,"cr":["701.24d"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Forest"],"hand":["Renewing Touch"],"graveyard":["Grizzly Bears"],` +
		`"library_top":["Forest","Plains","Plains","Swamp","Island"],"life":20},` +
		`"p1":{"library_top":["Plains","Island","Swamp","Mountain","Forest"],"life":20}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Renewing Touch","mana":"G","targets":["p0:Grizzly Bears"]},` +
		`{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	after := emptySetShuffleSnapshot(t, res, "step 1 (resolve)")
	if slices.Contains(after.Players[0].Graveyard, "Grizzly Bears") {
		t.Fatalf("p0 graveyard after the one-target cast = %v, want Grizzly Bears moved to the library", after.Players[0].Graveyard)
	}
	setup := []string{"Forest", "Plains", "Plains", "Swamp", "Island"}
	if slices.Equal(after.Players[0].LibraryTop, setup) {
		t.Fatalf("p0 library top after a one-target Renewing Touch = %v, want a shuffled library", after.Players[0].LibraryTop)
	}
}

func TestFarmDwellOnThePast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm dwell on the past zero card targets","turn":1,"cr":["701.24d"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Forest"],"hand":["Dwell on the Past"],` +
		`"library_top":["Forest","Plains","Plains","Swamp","Island"],"life":20},` +
		`"p1":{"graveyard":["Grizzly Bears"],"library_top":["Plains","Island","Swamp","Mountain","Forest"],"life":20}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Dwell on the Past","mana":"G","targets":["p1"]},` +
		`{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	before := emptySetShuffleSnapshot(t, res, "step 0 (cast)")
	if !slices.Contains(before.Players[1].Graveyard, "Grizzly Bears") {
		t.Fatalf("precondition: p1 graveyard after the cast = %v, want Grizzly Bears (the zero-card-target set)", before.Players[1].Graveyard)
	}
	after := emptySetShuffleSnapshot(t, res, "step 1 (resolve)")
	setup := []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	if slices.Equal(after.Players[1].LibraryTop, setup) {
		t.Fatalf("p1 library top after a zero-card-target Dwell on the Past = %v, want the targeted player's library shuffled (CR 701.24d)", after.Players[1].LibraryTop)
	}
}

func TestFarmTurnTheEarth(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm turn the earth zero targets","turn":1,"cr":["701.24d"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Forest"],"hand":["Turn the Earth"],"graveyard":["Grizzly Bears"],` +
		`"library_top":["Forest","Plains","Plains","Swamp","Island"],"life":20},` +
		`"p1":{"graveyard":["Grizzly Bears"],"library_top":["Plains","Island","Swamp","Mountain","Forest"],"life":20}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Turn the Earth","mana":"G"},` +
		`{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	after := emptySetShuffleSnapshot(t, res, "step 1 (resolve)")
	if got, want := after.Players[0].LibraryTop, []string{"Forest", "Plains", "Plains", "Swamp", "Island"}; !slices.Equal(got, want) {
		t.Fatalf("p0 library top after a zero-target Turn the Earth = %v, want the setup order %v: with no cards chosen the effect names no library", got, want)
	}
	if got, want := after.Players[1].LibraryTop, []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}; !slices.Equal(got, want) {
		t.Fatalf("p1 library top after a zero-target Turn the Earth = %v, want the setup order %v", got, want)
	}
}
