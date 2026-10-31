package pickbot_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestRatingFDNMainSetFullyRated pins that every distinct card name on the
// FDN booster spec's sheets has a 17Lands rating: 286 names (every printing
// a pack can carry, special-guest sheets included — 17Lands rates the SPG
// guests under FDN), all present in the embedded file.
func TestRatingFDNMainSetFullyRated(t *testing.T) {
	if !pb.RatedSet("FDN") {
		t.Fatal("no rating file for FDN")
	}
	if pb.RatedSet("ZZZ") {
		t.Fatal("ZZZ unexpectedly has a rating file")
	}
	if pb.Attribution("FDN") == "" {
		t.Fatal("the FDN rating file carries no attribution for the UI")
	}
	if pb.Attribution("ZZZ") != "" {
		t.Fatal("an unrated set returned an attribution")
	}
	names := fdnMainSetNames(t)
	if len(names) != 286 {
		t.Fatalf("FDN main set has %d distinct names, want 286", len(names))
	}
	missing := 0
	for _, n := range names {
		if _, ok := pb.RawRating("FDN", n); !ok {
			t.Errorf("FDN main-set card %q has no rating", n)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d of %d FDN main-set cards unrated", missing, len(names))
	}
}

// TestRatingPaths17LandsAndHeuristic pins the two paths Rating selects: the
// embedded 17Lands value for a rated set, the heuristic for any other.
func TestRatingPaths17LandsAndHeuristic(t *testing.T) {
	wr, ok := pb.RawRating("FDN", "Abrade")
	if !ok {
		t.Fatal("Abrade missing from the FDN rating file")
	}
	if got := pb.Rating("FDN", ref("a1", "Abrade", 0), nil); got != wr.GihWr {
		t.Fatalf("Rating(FDN, Abrade) = %g, want the file's %g", got, wr.GihWr)
	}
	if _, ok := pb.RawRating("ZZZ", "Savannah Lions"); ok {
		t.Fatal("ZZZ unexpectedly rated Savannah Lions")
	}
	reg := corpus(t)
	want := pb.Heuristic(lookupCard(t, reg, "Savannah Lions"), limited.Uncommon)
	if want == 0 {
		t.Fatal("heuristic scored Savannah Lions 0: the lookup or the card facts are wrong")
	}
	refr := limited.CardRef{Name: "Savannah Lions", Rarity: limited.Uncommon}
	if got := pb.Rating("ZZZ", refr, lookupOf(reg)); got != want {
		t.Fatalf("Rating(ZZZ, Savannah Lions) = %g, want the heuristic's %g", got, want)
	}
}
