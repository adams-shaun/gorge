package pickbot_test

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"

	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// trkUnratedSpells pins the non-land TRK booster-sheet names the simulated
// card-strength list never ranked: they keep the heuristic (or unscripted)
// path. A name leaving this list means the list was re-published to cover
// it; a name joining it means the list lost a card.
var trkUnratedSpells = []string{
	"Captain Benjamin Sisko",
	"Dathon, His Heart Open",
	"Isshin, Two Heavens as One",
	"Kenrith, the Returned King",
	"Riku of Many Paths",
	"Sarek, Federation Ambassador",
	"Sen Triplets",
	"Sisay, Weatherlight Captain",
}

// TestRatingTRKSimulatedPrior pins TRK's rating file: every row of the
// published simulated list rates its card at the row's prior_gih_wr, every
// non-land TRK booster-sheet card is rated or pinned unrated above, and
// RatingSource names the three sources.
func TestRatingTRKSimulatedPrior(t *testing.T) {
	if !pb.RatedSet("TRK") {
		t.Fatal("no rating file for TRK")
	}
	if pb.Attribution("TRK") == "" {
		t.Fatal("the TRK rating file carries no attribution")
	}
	f, err := os.Open("../../../docs/site/research/card-strength/TRK.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	if len(rows)-1 != 292 {
		t.Fatalf("TRK.csv has %d card rows, want 292", len(rows)-1)
	}
	for _, r := range rows[1:] {
		name := r[col["card"]]
		want, err := strconv.ParseFloat(r[col["prior_gih_wr"]], 64)
		if err != nil {
			t.Fatalf("%s: prior_gih_wr: %v", name, err)
		}
		got, ok := pb.RawRating("TRK", name)
		if !ok {
			t.Errorf("TRK list card %q has no rating", name)
			continue
		}
		if got.GihWr != want {
			t.Errorf("RawRating(TRK, %q) = %g, want the list's prior_gih_wr %g", name, got.GihWr, want)
		}
	}

	reg := corpus(t)
	pinned := map[string]bool{}
	for _, n := range trkUnratedSpells {
		pinned[n] = true
	}
	var unrated []string
	spells := 0
	for _, n := range trkMainSetNames(t) {
		c := lookupCard(t, reg, n)
		if c.Faces[0].IsLand() {
			continue
		}
		spells++
		if _, ok := pb.RawRating("TRK", n); ok {
			if pinned[n] {
				t.Errorf("TRK card %q is pinned unrated but the file rates it", n)
			}
			continue
		}
		unrated = append(unrated, n)
		if !pinned[n] {
			t.Errorf("non-land TRK card %q is unrated and not pinned", n)
		}
	}
	if spells < 250 {
		t.Fatalf("only %d non-land TRK sheet names: the sheet walk or the land test is wrong", spells)
	}
	if len(unrated) != len(trkUnratedSpells) {
		t.Errorf("unrated non-land TRK names %q, pinned %q", unrated, trkUnratedSpells)
	}

	for set, want := range map[string]string{"FDN": "17lands", "TRK": "simulated", "BLB": "heuristic"} {
		if got := pb.RatingSource(set); got != want {
			t.Errorf("RatingSource(%s) = %q, want %q", set, got, want)
		}
	}
}
