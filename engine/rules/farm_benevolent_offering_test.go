package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func offeringSnap(t *testing.T, sc, checkpoint string) OracleSnapshot {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	for _, s := range res.Snapshots {
		if s.Checkpoint == checkpoint {
			return s
		}
	}
	t.Fatalf("no %q snapshot", checkpoint)
	return OracleSnapshot{}
}

// TestFarmBenevolentOffering pins the SUP-s336-supp farm window: Forge's
// leading-dot player selector `.Chosen,You` (TokenOwner$) is every player
// filtered by Player.Chosen OR Player.You, so the caster and the chosen
// opponent each create the three Spirits (CR 111.2). gorge sent them all to
// the caster.
func TestFarmBenevolentOffering(t *testing.T) {
	t.Parallel()
	bears := `"Grizzly Bears","Grizzly Bears","Grizzly Bears","Grizzly Bears"`
	sc := `{"name":"farm benevolent offering","turn":1,"cr":["111.2"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Plains","Plains","Plains","Plains",` + bears + `],"hand":["Benevolent Offering"],"life":10},` +
		`"p1":{"battlefield":[` + bears + `],"life":10}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Plains","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Plains#2","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Plains#3","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Plains#4","ability_index":0},` +
		`{"op":"cast","seat":0,"card":"p0:Benevolent Offering"},{"op":"resolve"}]}`
	snap := offeringSnap(t, sc, "step 5 (resolve)")
	creatures := [2]int{}
	for _, p := range snap.Permanents {
		if p.PT != "" && p.Controller < 2 {
			creatures[p.Controller]++
		}
	}
	if creatures != [2]int{7, 7} {
		t.Fatalf("creatures = %v, want [7 7] (both seats create three Spirits)", creatures)
	}
	for _, pl := range snap.Players {
		if pl.Life != 24 {
			t.Fatalf("seat %d life = %d, want 24", pl.Seat, pl.Life)
		}
	}
}

// TestFarmIntellectualOffering: Defined$ .Chosen,You draws for both players.
func TestFarmIntellectualOffering(t *testing.T) {
	t.Parallel()
	lib := `"Island","Island","Island","Island","Island"`
	sc := `{"name":"farm intellectual offering","turn":1,"cr":["121.1"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Island","Island","Island","Island","Island"],"hand":["Intellectual Offering"],"library":[` + lib + `]},` +
		`"p1":{"library":[` + lib + `]}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Island","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Island#2","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Island#3","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Island#4","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Island#5","ability_index":0},` +
		`{"op":"cast","seat":0,"card":"p0:Intellectual Offering"},{"op":"resolve"}]}`
	snap := offeringSnap(t, sc, "step 6 (resolve)")
	for _, pl := range snap.Players {
		if len(pl.Hand) != 3 {
			t.Fatalf("seat %d hand = %v, want 3 cards", pl.Seat, pl.Hand)
		}
	}
}
