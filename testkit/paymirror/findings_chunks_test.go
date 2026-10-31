package paymirror

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The round-N findings mirrors (TestRecordedFindingsMirrorKernelChunkNN,
// TestRecordedFindingsMirrorChunkNN) replay a file of finding games one
// chunk per test: run whole, each round played its four games serially in
// one test, ~15 s alone at 4 vCPU and 31.6 s under gate contention
// (2026-10-12). Chunk i of K plays specs[i*n/K : (i+1)*n/K] in file order,
// so the chunks partition the file exactly -- a spec added to the file
// grows a chunk and is never dropped (checkFindingPartition).

// findingDecksMemo is LoadDecks over the process-shared registry, built
// once per test binary for every findings chunk.
var findingDecksMemo struct {
	sync.Once
	d   *Decks
	err error
}

func findingDecks(t *testing.T) *Decks {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	findingDecksMemo.Do(func() { findingDecksMemo.d, findingDecksMemo.err = LoadDecks(reg) })
	if findingDecksMemo.err != nil {
		t.Fatal(findingDecksMemo.err)
	}
	return findingDecksMemo.d
}

// findingSpecs reads a findings file's game specs in file order and holds
// it to want specs.
func findingSpecs(t *testing.T, path string, want int) []GameSpec {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var specs []GameSpec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var s GameSpec
		if err := json.Unmarshal(sc.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		specs = append(specs, s)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != want {
		t.Fatalf("testdata holds %d specs, want %d", len(specs), want)
	}
	return specs
}

// findingChunk returns chunk i of k.
func findingChunk(specs []GameSpec, k, i int) []GameSpec {
	n := len(specs)
	return specs[i*n/k : (i+1)*n/k]
}

// checkFindingPartition holds a round's sharding to full coverage: one chunk
// test per chunk, no chunk empty, and the chunks, concatenated, are exactly
// the file's specs in order.
func checkFindingPartition(t *testing.T, specs []GameSpec, k int, tests []func(*testing.T)) {
	t.Helper()
	if len(tests) != k {
		t.Fatalf("%d chunk tests for %d chunks", len(tests), k)
	}
	var joined []GameSpec
	for i := 0; i < k; i++ {
		c := findingChunk(specs, k, i)
		if len(c) == 0 {
			t.Errorf("chunk %d of %d specs is empty", i, len(specs))
		}
		joined = append(joined, c...)
	}
	if len(joined) != len(specs) {
		t.Fatalf("chunks cover %d of %d specs", len(joined), len(specs))
	}
	for i := range specs {
		if joined[i].Seed != specs[i].Seed {
			t.Fatalf("chunk concatenation diverges at %d: seed %d vs %d", i, joined[i].Seed, specs[i].Seed)
		}
	}
}
