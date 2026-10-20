package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRoundSixFindingsMirrorKernel replays the round-6 finding games end to
// end on the resolution kernel: no cast is a mismatch, the live-vs-clone
// control is equivalent everywhere, and each named cast gets its root-caused
// verdict (the history of every pin is in the deleted
// TestRoundSixFindingsMirror's doc, round6_test.go at 19fa2a40d):
//
//   - 4130: control contChain.len; the Firebird cast no longer occurs, so the
//     seed keeps an empty pin and asserts a clean, control-equivalent game;
//   - 2138: admitting Phyrexian costs to auto-pay changes the bot's cast
//     trajectory, so the old seq-1488 float-trigger finding is no longer
//     reached; the seed remains a clean, control-equivalent game;
//   - 25 seq 269: Master of Dark Rites, equivalent (same deck set as the old
//     4098 pin). The Dig absent-ChangeNum default now moves one card rather
//     than the whole window, changing the preceding game's event count
//     (formerly seq 7684). Ticket agent-20261006T052735Z-5c29adb5:
//     Necropotence's Defined$ TopOfLibrary exile is a position read and no
//     longer emits a mandatory shuffle (CR 401.4), shifting the same cast from
//     seq 7830 to 7795; the paid-cost census default-on flip
//     (cli-20261012T170859Z) then displaced the cast from seed 4098 entirely
//     (no planned cast of it remains), so the pin is re-seeded to 25, where the
//     same deck set reaches it at seq 269, verdict equivalent; the combat
//     priority fixes (rules e7aaf1530 and 93b6057e4, CR 510.4/117.3a: more
//     priority windows in combat) move it to seq 282;
//   - 4139: CR 601.2c now announces graveyard ChangeZone sub-targets during
//     casting, changing the bot game's path before the Urza's Incubator
//     finding. The seed remains pinned as a clean, control-equivalent game;
//     the old cast-specific finding is no longer reached after the rules fix.
//   - 4129 seq 3318: Demonic Tutor, equivalent (the Ogre pay-life-only
//     witness stays pinned by TestWitnessReadsPayLifeOnlySources). Ticket
//     agent-20261012T222541Z-ceca6383: pro-shaper's Exotic Orchard is now a
//     planned source (an opponent-reflecting ManaReflected colour choice), so
//     the bot's auto-pay casts earlier through it and the same cast moves from
//     seq 5797 to 5740 (confirmed by reverting the planner hunk: 5797 returns);
//     the paid-cost census default-on flip (cli-20261012T170859Z) then moved it
//     to seq 5755, and botmanual (2026-10-14: the auto-pay bot's tap gate reads the engine's cast census, and flashback and fixed-discard casts get payment plans)
//     to seq 3318, the combat priority fixes (rules e7aaf1530,
//     93b6057e4) to seq 3500, and the CR 510.3 post-damage round (rules
//     24bbba791) to seq 3532, and the mandatory library-search shuffle
//     (cr-g09: the Flamekin Harbinger may-shuffle ask is gone, two fewer
//     events) to seq 3530, verdict still equivalent.
func TestRoundSixFindingsMirrorKernel(t *testing.T) {
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
		{2138, []string{"vivi-ornitier-cedh", "hearthhull-worldseed-landfall", "pro-shaper", "foundations-keen-engineering"}, 0, ""},
		{25, []string{"foundations-reign-of-dragons", "hearthhull-worldseed-landfall", "avengers-assemble", "rakdos-muscle-scam-exe"}, 282, ""},
		{4139, []string{"foundations-wretched-ranks", "deadly-disguise", "foundations-reign-of-dragons", "ulalek-eldrazi"}, 0, ""},
		{4129, []string{"rakdos-muscle-scam-exe", "pro-shaper", "foundations-reign-of-dragons", "foundations-wretched-ranks"}, 3530, ""},
	} {
		reports := round6Game(t, d, GameSpec{Seed: tc.seed, Decks: tc.decks, Commander: true, Policy: "bot"})
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
