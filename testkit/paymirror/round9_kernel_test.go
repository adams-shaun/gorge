package paymirror

import "testing"

// TestRoundNineFindingsMirrorKernel (TestRoundNineFindingsMirrorKernelChunkNN,
// sharded per findings_chunks_test.go) replays the round-9 paymirror finding
// games (testdata/round9.jsonl: repo deck names or random card-name lists,
// no script text) end to end on the resolution kernel: no cast is a
// mismatch, the live-vs-clone control is equivalent, and each named cast
// gets its root-caused verdict (the history of every pin is in the deleted
// TestRoundNineFindingsMirror's doc, round9_test.go at 19fa2a40d):
//
//   - 10860 seq 4637: Worldly Tutor (the Command Tower production fix),
//     equivalent (was seq 3239 until eng-turnwatch: an aborted cast now
//     logs a SpellCastReversed event, shifting every later seq by +7 here;
//     the card list and verdicts of the game are otherwise identical; then
//     3246 until botmanual (2026-10-14: the auto-pay bot's tap gate reads the engine's cast census, and flashback and fixed-discard casts get payment plans) moved the
//     bots' trajectory, same card and verdict; then 4637 until the combat
//     priority fixes (rules e7aaf1530 and 93b6057e4, CR 510.4/117.3a: more
//     priority windows in combat) moved it to seq 4897, and the CR 510.3
//     post-damage round (rules 24bbba791) to seq 4929, and cr-g69 (CR
//     601.2h: one duplicate Priority event fewer when a cost answer completes
//     a cast) to seq 4928, same card and verdict; then cr-g125 (2026-10-26:
//     an untapped RestrictValid$ source now funds a planned cast its
//     restriction admits, so the bots' trajectory moves) to seq 4916,
//     confirmed by reverting the planner hunk: 4928 returns, same card and
//     verdict; then the X-bound target offer (CR 601.2c: a cast with no possible
//     target is no longer offered) to seq 4866, same card and verdict; then that
//     offer pricing its X (CR 601.2b) to seq 4681, same card and verdict; then an
//     effect's tapped entry yielding to "enters untapped" (CR 614.12: one Tap event
//     fewer) to seq 4680, confirmed by undoing that check: 4681 returns, same card
//     and verdict);
//   - 11056: the Artisan finding is no longer reached; a clean,
//     control-equivalent game;
//   - 10056: Lagomos, Hand of Hatred was equivalent at seq 6750 (the
//     paid-cost census default-on flip, cli-20261012T170859Z, had moved it
//     from seq 6128); botmanual (2026-10-14: the auto-pay bot's tap gate reads the engine's cast census, and flashback and fixed-discard casts get payment plans) moves the
//     game so the cast is no longer reached; a clean, control-equivalent
//     game;
//   - 8175: the Hideaway ETB timing correction changes this game before the
//     former Songs of the Damned finding, so that planned cast is no longer
//     reached (verified against the pre-fix keyword expansion).
//
// roundNineWant holds those pins.
var roundNineWant = map[uint64]map[uint64]string{ // seed -> seq -> verdict key ("" = equivalent)
	10860: {4680: ""},
	11056: {},
	10056: {},
	8175:  {},
}

const (
	roundNineFile   = "testdata/round9.jsonl"
	roundNineChunks = 4
)

func TestRoundNineFindingsMirrorKernelChunk00(t *testing.T) { runRoundNineChunk(t, 0) }
func TestRoundNineFindingsMirrorKernelChunk01(t *testing.T) { runRoundNineChunk(t, 1) }
func TestRoundNineFindingsMirrorKernelChunk02(t *testing.T) { runRoundNineChunk(t, 2) }
func TestRoundNineFindingsMirrorKernelChunk03(t *testing.T) { runRoundNineChunk(t, 3) }

// TestRoundNineFindingsMirrorKernelChunksPartitionTheSpecs: the chunks play
// every round-9 spec exactly once (checkFindingPartition).
func TestRoundNineFindingsMirrorKernelChunksPartitionTheSpecs(t *testing.T) {
	checkFindingPartition(t, findingSpecs(t, roundNineFile, 4), roundNineChunks, []func(*testing.T){
		TestRoundNineFindingsMirrorKernelChunk00, TestRoundNineFindingsMirrorKernelChunk01,
		TestRoundNineFindingsMirrorKernelChunk02, TestRoundNineFindingsMirrorKernelChunk03})
}

// runRoundNineChunk is one shard of TestRoundNineFindingsMirrorKernel.
func runRoundNineChunk(t *testing.T, i int) {
	t.Parallel()
	d := findingDecks(t)
	for _, spec := range findingChunk(findingSpecs(t, roundNineFile, 4), roundNineChunks, i) {
		reports := round6Game(t, d, spec)
		if len(reports) == 0 {
			t.Errorf("seed %d: no planned-cast reports; clean-game assertions would be vacuous", spec.Seed)
		}
		seen := map[uint64]bool{}
		for _, r := range reports {
			st, key := r.Verdict()
			if st == Mismatch || (st == Unmirrorable && !r.ExpectedUnmirrorable()) {
				t.Errorf("seed %d seq %d %q: %s %s", spec.Seed, r.Seq, r.Card, st, key)
			}
			if r.Control == nil || r.Control.Status != Equivalent {
				t.Errorf("seed %d seq %d %q: control %+v", spec.Seed, r.Seq, r.Card, r.Control)
			}
			w, ok := roundNineWant[spec.Seed][r.Seq]
			if !ok {
				continue
			}
			seen[r.Seq] = true
			if key != w {
				t.Errorf("seed %d seq %d %q: verdict %s %q, want %q", spec.Seed, r.Seq, r.Card, st, key, w)
			}
		}
		for seq := range roundNineWant[spec.Seed] {
			if !seen[seq] {
				t.Errorf("seed %d: no planned cast at seq %d (the game no longer reaches the finding)", spec.Seed, seq)
			}
		}
	}
}
