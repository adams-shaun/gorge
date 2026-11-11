package paymirror

import "testing"

// TestRecordedFindingsMirrorKernel (TestRecordedFindingsMirrorKernelChunkNN,
// sharded per findings_chunks_test.go) replays recorded paymirror finding
// games (testdata/findings_recorded_kernel.jsonl: repo deck names or random
// card-name lists, no script text) end to end on the resolution kernel: no
// cast is a mismatch, the live-vs-clone control is equivalent, and each named
// cast gets its root-caused verdict:
//
//   - 10860 seq 4389: Worldly Tutor (the Command Tower production fix),
//     equivalent. The seq moves whenever a rule change shifts this game's log or
//     the bots' trajectory; a move keeps the card and the verdict;
//   - 11056: the Artisan finding is no longer reached; a clean,
//     control-equivalent game;
//   - 10056: the Lagomos, Hand of Hatred cast is no longer reached; a clean,
//     control-equivalent game;
//   - 8175: the Hideaway ETB timing correction changes this game before the
//     former Songs of the Damned finding, so that planned cast is no longer
//     reached (checked against the expansion without that timing).
//
// recordedKernelWant holds those pins.
var recordedKernelWant = map[uint64]map[uint64]string{ // seed -> seq -> verdict key ("" = equivalent)
	10860: {4389: ""},
	11056: {},
	10056: {},
	8175:  {},
}

const (
	recordedKernelFile   = "testdata/findings_recorded_kernel.jsonl"
	recordedKernelChunks = 4
)

func TestRecordedFindingsMirrorKernelChunk00(t *testing.T) { runRecordedKernelChunk(t, 0) }
func TestRecordedFindingsMirrorKernelChunk01(t *testing.T) { runRecordedKernelChunk(t, 1) }
func TestRecordedFindingsMirrorKernelChunk02(t *testing.T) { runRecordedKernelChunk(t, 2) }
func TestRecordedFindingsMirrorKernelChunk03(t *testing.T) { runRecordedKernelChunk(t, 3) }

// TestRecordedFindingsMirrorKernelChunksPartitionTheSpecs: the chunks play
// every recorded spec exactly once (checkFindingPartition).
func TestRecordedFindingsMirrorKernelChunksPartitionTheSpecs(t *testing.T) {
	checkFindingPartition(t, findingSpecs(t, recordedKernelFile, 4), recordedKernelChunks, []func(*testing.T){
		TestRecordedFindingsMirrorKernelChunk00, TestRecordedFindingsMirrorKernelChunk01,
		TestRecordedFindingsMirrorKernelChunk02, TestRecordedFindingsMirrorKernelChunk03})
}

// runRecordedKernelChunk is one shard of TestRecordedFindingsMirrorKernel.
func runRecordedKernelChunk(t *testing.T, i int) {
	t.Parallel()
	d := findingDecks(t)
	for _, spec := range findingChunk(findingSpecs(t, recordedKernelFile, 4), recordedKernelChunks, i) {
		reports := findingGame(t, d, spec)
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
			w, ok := recordedKernelWant[spec.Seed][r.Seq]
			if !ok {
				continue
			}
			seen[r.Seq] = true
			if key != w {
				t.Errorf("seed %d seq %d %q: verdict %s %q, want %q", spec.Seed, r.Seq, r.Card, st, key, w)
			}
		}
		for seq := range recordedKernelWant[spec.Seed] {
			if !seen[seq] {
				t.Errorf("seed %d: no planned cast at seq %d (the game no longer reaches the finding)", spec.Seed, seq)
			}
		}
	}
}
