package pickbot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// drafterCards are the two candidates the colour-preference case turns on:
// a green rare rated higher than a black rare by one heuristic point
// (Spinner of Souls 4/3 for 3 vs Abyssal Harvester 3/2 for 3), close enough
// that the colour term can flip the order once black is established.
func drafterCards(t *testing.T) (green, black limited.CardRef) {
	t.Helper()
	return ref("g1", "Spinner of Souls", limited.Rare), ref("b1", "Abyssal Harvester", limited.Rare)
}

// blackPicks is eight black cards, the seat's pool when the colour
// preference is asserted.
func blackPicks() []limited.CardRef {
	names := []string{
		"Diregraf Ghoul", "Vampire Nighthawk", "Stromkirk Bloodthief",
		"Vampire Gourmand", "Reassembling Skeleton", "Billowing Shriekmass",
		"Nine-Lives Familiar", "Crypt Feaster",
	}
	picks := make([]limited.CardRef, len(names))
	for i, n := range names {
		picks[i] = ref("p"+itoa(i), n, limited.Uncommon)
	}
	return picks
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// TestDrafterPicksHighestRated pins the rated path with a zero-value
// drafter: at pack 1 pick 1 the colour term is still ramped to zero, so the
// pick is the highest 17Lands rating in the pack, no corpus needed.
func TestDrafterPicksHighestRated(t *testing.T) {
	d := &pb.RatingColour{}
	pack := []limited.CardRef{
		ref("s1", "Thousand-Year Storm", limited.Rare),         // gih_wr 0.3479
		ref("l1", "Liliana, Dreadhorde General", limited.Rare), // gih_wr 0.6412
	}
	v := pb.PickView{Set: "FDN", Round: 1, PickNo: 1, Pack: pack}
	if got := d.Pick(v, rand.New(rand.NewPCG(1, 2))); got != "l1" {
		t.Fatalf("empty-pool pick = %q, want l1 (the higher 17Lands rating)", got)
	}
}

// TestDrafterPrefersColourAfterEightPicks constructs the spec case: with an
// empty pool the drafter picks the higher-rated green card; after eight
// black picks it prefers the slightly lower-rated black card, because the
// colour term at pick 9 outweighs the one-point rating gap.
func TestDrafterPrefersColourAfterEightPicks(t *testing.T) {
	reg := corpus(t)
	d := &pb.RatingColour{Lookup: lookupOf(reg)}
	green, black := drafterCards(t)

	first := pb.PickView{
		Set: "ZZZ", Round: 1, PickNo: 1,
		Pack: []limited.CardRef{green, black},
	}
	if got := d.Pick(first, rand.New(rand.NewPCG(3, 4))); got != green.ID {
		t.Fatalf("empty-pool pick = %q, want the higher-rated green card %q", got, green.ID)
	}

	ninth := pb.PickView{
		Set: "ZZZ", Round: 1, PickNo: 9,
		Pack:  []limited.CardRef{green, black},
		Picks: blackPicks(),
	}
	if got := d.Pick(ninth, rand.New(rand.NewPCG(3, 4))); got != black.ID {
		t.Fatalf("pick 9 after 8 black picks = %q, want the black card %q", got, black.ID)
	}
	// The gap is genuinely narrow: without the colour term the green card
	// still wins at pick 9 (precondition for the flip meaning anything).
	noColour := &pb.RatingColour{Lookup: lookupOf(reg), Tuning: pb.Tuning{
		RatingCentre: pb.DefaultTuning.RatingCentre, RatingScale: pb.DefaultTuning.RatingScale,
		HeuristicCentre: pb.DefaultTuning.HeuristicCentre, HeuristicScale: pb.DefaultTuning.HeuristicScale,
		ColourFitMax: 0, ColourFitRamp: pb.DefaultTuning.ColourFitRamp,
		ColourlessFit: pb.DefaultTuning.ColourlessFit, MainSize: pb.DefaultTuning.MainSize,
		DeckSize: pb.DefaultTuning.DeckSize, MinCreatures: pb.DefaultTuning.MinCreatures,
		HighValueMV: pb.DefaultTuning.HighValueMV, MaxHighValue: pb.DefaultTuning.MaxHighValue,
	}}
	if got := noColour.Pick(ninth, rand.New(rand.NewPCG(3, 4))); got != green.ID {
		t.Fatalf("with the colour term disabled, pick 9 = %q, want the higher-rated green card %q", got, green.ID)
	}
}

// TestDrafterTieBrokenByRng pins that an exact tie — the same card twice in
// one pack — resolves through the seeded rng and is reproducible.
func TestDrafterTieBrokenByRng(t *testing.T) {
	reg := corpus(t)
	d := &pb.RatingColour{Lookup: lookupOf(reg)}
	pack := []limited.CardRef{
		ref("t1", "Healer's Hawk", limited.Common),
		ref("t2", "Healer's Hawk", limited.Common),
	}
	v := pb.PickView{Set: "ZZZ", Round: 1, PickNo: 5, Pack: pack}
	got := d.Pick(v, rand.New(rand.NewPCG(7, 8)))
	if got != "t1" && got != "t2" {
		t.Fatalf("tied pick = %q, want one of the pack's IDs", got)
	}
	if again := d.Pick(v, rand.New(rand.NewPCG(7, 8))); again != got {
		t.Fatalf("same seed resolved the tie differently: %q then %q", got, again)
	}
}

// TestRatingColourScoresMatchPick: Pick is exactly BestScore over Scores.
func TestRatingColourScoresMatchPick(t *testing.T) {
	d := &pb.RatingColour{}
	pack := []limited.CardRef{{ID: "a", Name: "Llanowar Elves", Set: "FDN"}, {ID: "b", Name: "Shivan Dragon", Set: "FDN"}, {ID: "c", Name: "Giant Growth", Set: "FDN"}}
	v := pb.PickView{Set: "FDN", Round: 2, PickNo: 3, Pack: pack}
	sc := d.Scores(v, nil)
	if len(sc) != len(pack) {
		t.Fatalf("%d scores for %d cards", len(sc), len(pack))
	}
	r1, r2 := rand.New(rand.NewPCG(3, 3)), rand.New(rand.NewPCG(3, 3))
	if got, want := d.Pick(v, r1), pack[pb.BestScore(sc, r2)].ID; got != want {
		t.Fatalf("Pick %s, BestScore %s", got, want)
	}
	if pb.BestScore(nil, nil) != -1 {
		t.Fatal("BestScore of nothing")
	}
}
