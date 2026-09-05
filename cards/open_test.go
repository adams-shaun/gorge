package cards

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeScript writes one synthetic card script into dir/cardsfolder. The
// text is authored here; it is not a Forge file.
func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	folder := CorpusDir(dir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCorpusFailsCleanlyWhenNothingIsThere(t *testing.T) {
	if _, err := OpenCorpus(t.TempDir()); err == nil {
		t.Fatal("OpenCorpus on an empty dir returned no error")
	}
}

func TestOpenCorpusCompilesWhenThereIsNoCache(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	r, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("Mountain"); !ok {
		t.Fatal("compiled registry lacks Mountain")
	}
}

func TestOpenCorpusPrefersAFreshCacheAndRecompilesAStaleOne(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	// A cache holding a different card: fresh cache wins over the folder.
	r := NewRegistry()
	c, _ := ParseBytes("island.txt", []byte("Name:Island\nTypes:Basic Land Island\nOracle:\n"))
	r.Add(c)
	cache := filepath.Join(dir, "ir.gob.gz")
	if err := r.Save(cache); err != nil {
		t.Fatal(err)
	}
	got, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Lookup("Island"); !ok {
		t.Fatal("fresh cache was not used")
	}
	// Now a cards.lock newer than the cache marks it stale: recompile.
	lock := filepath.Join(dir, "cards.lock")
	if err := os.WriteFile(lock, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(lock, future, future); err != nil {
		t.Fatal(err)
	}
	got, err = OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Lookup("Mountain"); !ok {
		t.Fatal("stale cache was not recompiled from the folder")
	}
}
