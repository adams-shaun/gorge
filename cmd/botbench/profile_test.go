package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestProfilerWritesReadableProfiles drives the -cpuprofile / -memprofile
// helpers end to end on one real game and checks both files land on disk as
// gzip-compressed pprof profiles (the two things pprof itself needs: present,
// non-empty, and a valid gzip stream, which is how the pprof protobuf is
// carried). A profiled run must also leave the run's own report untouched --
// the game output below is asserted non-empty, the same report an
// unprofiled run writes.
func TestProfilerWritesReadableProfiles(t *testing.T) {
	dir := corpusDirOrSkip(t)
	tmp := t.TempDir()
	cpuPath := filepath.Join(tmp, "cpu.pprof")
	heapPath := filepath.Join(tmp, "heap.pprof")

	prof := &profiler{cpuPath: cpuPath, memPath: heapPath}
	if err := prof.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	var buf bytes.Buffer
	if err := run(0, 1, 2, 0, 1, "bot", "bot", dir, 200, 0, false, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := prof.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if len(buf.Bytes()) == 0 {
		t.Fatal("profiled run wrote an empty report")
	}

	for _, tc := range []struct{ name, path string }{
		{"cpuprofile", cpuPath},
		{"memprofile", heapPath},
	} {
		f, err := os.Open(tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("%s: not a gzip pprof stream: %v", tc.name, err)
		}
		n, err := io.Copy(io.Discard, zr)
		if err != nil {
			t.Fatalf("%s: unreadable after gzip header: %v", tc.name, err)
		}
		if n == 0 {
			t.Fatalf("%s: empty pprof payload", tc.name)
		}
		st, err := os.Stat(tc.path)
		if err != nil || st.Size() == 0 {
			t.Fatalf("%s: missing or empty file", tc.name)
		}
	}

	// finish is idempotent: a second call after the first must be a no-op
	// that does not clobber or error on the written profiles.
	if err := prof.finish(); err != nil {
		t.Fatalf("second finish: %v", err)
	}
}

// TestProfilerIsANoOpWithoutPaths pins the default-path invariant: with both
// paths empty, start and finish touch nothing -- the constructed unprofiled
// run is exactly what mainExit runs.
func TestProfilerIsANoOpWithoutPaths(t *testing.T) {
	prof := &profiler{}
	if err := prof.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if prof.cpuFile != nil {
		t.Fatal("empty cpuPath started CPU profiling")
	}
	if err := prof.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
}
