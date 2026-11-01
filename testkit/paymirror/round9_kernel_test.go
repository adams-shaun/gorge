package paymirror

import "testing"

// TestRoundNineFindingsMirrorKernel (TestRoundNineFindingsMirrorKernelChunkNN,
// sharded per findings_chunks_test.go) replays the round-9 paymirror finding
// games (testdata/round9.jsonl: repo deck names or random card-name lists,
// no script text) end to end on the resolution kernel: no cast is a
// mismatch, the live-vs-clone control is equivalent, and each named cast
// gets its root-caused verdict:
//
//   - 10860 seq 4745: Worldly Tutor (the Command Tower production fix),
//     equivalent. The seq moves whenever a rule change shifts this game's log or
//     the bots' trajectory; a move keeps the card and the verdict;
//   - 11056: the Artisan finding is no longer reached; a clean,
//     control-equivalent game;
//   - 10056: the Lagomos, Hand of Hatred cast is no longer reached; a clean,
//     control-equivalent game;
//   - 8175: the Hideaway ETB timing correction changes this game before the
//     former Songs of the Damned finding, so that planned cast is no longer
//     reached (verified against the pre-fix keyword expansion).
//
// roundNineWant holds those pins.
var roundNineWant = map[uint64]map[uint64]string{ // seed -> seq -> verdict key ("" = equivalent)
	10860: {4745: ""},
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
