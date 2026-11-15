package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCommanderFindingsMirrorKernel replays commander games that once showed a
// mirror finding, end to end on the resolution kernel: no cast is a mismatch,
// the live-vs-clone control is equivalent everywhere, and each named cast gets
// its root-caused verdict. A seq moves whenever a rule change shifts a game's log
// or the bots' trajectory; a move keeps the card and the verdict.
//
//   - 4130, 4139: the findings are no longer reached, so each seed keeps an
//     empty pin and asserts a clean, control-equivalent game;
//   - 2138 seq 11901: Kappa Cannoneer, equivalent. The plan credits Improvise
//     with five artifacts, the seat taps one, and the engine plans the cast
//     again (CR 702.126a, 601.2g-h); the float route floats that witness, the
//     one run A paid by (Report.paid);
//   - 25 seq 273: Master of Dark Rites, equivalent;
//   - 4129 seq 1381: Demonic Tutor, equivalent (the Ogre pay-life-only witness
//     stays pinned by TestWitnessReadsPayLifeOnlySources).
func TestCommanderFindingsMirrorKernel(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	d, err := LoadDecks(reg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		seed  uint64
		decks []string
		seq   uint64
		want  string // the named cast's verdict key ("" = equivalent)
	}{
		{4130, []string{"vivi-ornitier-cedh", "foundations-reign-of-dragons", "avengers-assemble", "valgavoth-endless-punishment"}, 0, ""},
		{2138, []string{"vivi-ornitier-cedh", "hearthhull-worldseed-landfall", "pro-shaper", "foundations-keen-engineering"}, 11901, ""},
		{25, []string{"foundations-reign-of-dragons", "hearthhull-worldseed-landfall", "avengers-assemble", "rakdos-muscle-scam-exe"}, 273, ""},
		{4139, []string{"foundations-wretched-ranks", "deadly-disguise", "foundations-reign-of-dragons", "ulalek-eldrazi"}, 0, ""},
		{4129, []string{"rakdos-muscle-scam-exe", "pro-shaper", "foundations-reign-of-dragons", "foundations-wretched-ranks"}, 1381, ""},
	} {
		reports := findingGame(t, d, GameSpec{Seed: tc.seed, Decks: tc.decks, Commander: true, Policy: "bot"})
		if len(reports) == 0 {
			t.Errorf("seed %d: no planned-cast reports; clean-game assertions would be vacuous", tc.seed)
		}
		found := false
		for _, r := range reports {
			st, key := r.Verdict()
			if st == Mismatch {
				t.Errorf("seed %d seq %d %q: %s (witness %q)", tc.seed, r.Seq, r.Card, key, r.AWitness)
			}
			if r.Control == nil || r.Control.Status != Equivalent {
				t.Errorf("seed %d seq %d %q: control %+v", tc.seed, r.Seq, r.Card, r.Control)
			}
			if r.Seq != tc.seq {
				continue
			}
			found = true
			if key != tc.want {
				t.Errorf("seed %d seq %d %q: verdict %s %q, want %q", tc.seed, r.Seq, r.Card, st, key, tc.want)
			}
		}
		if !found && tc.seq != 0 {
			t.Errorf("seed %d: no planned cast at seq %d (the game no longer reaches the finding)", tc.seed, tc.seq)
		}
	}
}
