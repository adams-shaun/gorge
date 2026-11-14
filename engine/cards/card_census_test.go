package cards

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCardCensusRefusesACacheFromAViewMissingAScript: a cache saved by a
// corpus view holding one FEWER card script is newer than every mtime of the
// reader's corpus (the script and its folders are backdated), so only the
// census sidecar can see the reader has a script the cache was not compiled
// from.
func TestCardCensusRefusesACacheFromAViewMissingAScript(t *testing.T) {
	dir := t.TempDir()
	writeLetterScript(t, dir, "b", "bog.txt", cardBody("Bog", "Artifact"))
	lock := writeOldLock(t, dir)
	folder := CorpusDir(dir)

	// The writer's view: Bog only. OpenCorpus compiles and saves the cache
	// and its census.
	first, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := first.Lookup("Bespoke Bo"); ok {
		t.Fatal("precondition: the writer's view already holds Bespoke Bo")
	}
	cache := CachePath(dir)
	if _, err := os.Stat(cardCensusPath(cache)); err != nil {
		t.Fatalf("precondition: Save wrote no census sidecar: %v", err)
	}

	// The reader's view: the same corpus plus one script, every corpus mtime
	// older than the cache.
	writeLetterScript(t, dir, "b", "bespoke.txt", cardBody("Bespoke Bo", "Artifact"))
	backdate(t, filepath.Join(folder, "b", "bog.txt"), filepath.Join(folder, "b", "bespoke.txt"),
		filepath.Join(folder, "b"), folder, lock)
	ci, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if corpusInputNewerThan(dir, ci) {
		t.Fatal("precondition: the mtime rule already sees the added script")
	}
	if si, err := os.Stat(SegmentPath(cache)); err != nil || corpusInputNewerThan(dir, si) {
		t.Fatalf("precondition: the segment is missing or stale by mtime (%v)", err)
	}

	if cacheFresh(dir, cache) {
		t.Fatal("cache saved without bespoke.txt is fresh for a corpus that has it")
	}
	if segFresh(dir, cache, SegmentPath(cache)) {
		t.Fatal("segment saved without bespoke.txt is fresh for a corpus that has it")
	}
	got, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Bog", "Bespoke Bo"} {
		if _, ok := got.Lookup(n); !ok {
			t.Fatalf("reopened corpus is missing %s", n)
		}
	}
	// The recompile recorded the reader's census, so the next open hits.
	if !cacheFresh(dir, cache) {
		t.Fatal("the recompiled cache is not fresh for the corpus it was compiled from")
	}
}

// TestCardCensusMissingSidecarDefersToTheMtimeRule: a cache with no census
// (written before the census existed) is judged by mtimes alone, so a
// read-only shared corpus does not recompile on every open.
func TestCardCensusMissingSidecarDefersToTheMtimeRule(t *testing.T) {
	dir := t.TempDir()
	writeLetterScript(t, dir, "b", "bog.txt", cardBody("Bog", "Artifact"))
	lock := writeOldLock(t, dir)
	folder := CorpusDir(dir)
	backdate(t, filepath.Join(folder, "b", "bog.txt"), filepath.Join(folder, "b"), folder, lock)
	if _, err := OpenCorpus(dir); err != nil {
		t.Fatal(err)
	}
	cache := CachePath(dir)
	if !cacheFresh(dir, cache) {
		t.Fatal("precondition: the saved cache is not fresh")
	}
	writeLetterScript(t, dir, "b", "bespoke.txt", cardBody("Bespoke Bo", "Artifact"))
	backdate(t, filepath.Join(folder, "b", "bespoke.txt"), filepath.Join(folder, "b"))
	if cacheFresh(dir, cache) {
		t.Fatal("precondition: the census does not see the added script")
	}
	if err := os.Remove(cardCensusPath(cache)); err != nil {
		t.Fatal(err)
	}
	if !cacheFresh(dir, cache) {
		t.Fatal("a cache without a census sidecar is refused though its mtimes are fresh")
	}
}
