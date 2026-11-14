package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

// cards-overlay.sh makes cardsfolder a symlink; the ALL list must still see its
// scripts, not read as an empty tree.
func TestCorpusAllNamesThroughSymlinkedRoot(t *testing.T) {
	real := filepath.Join(t.TempDir(), "cardsfolder")
	if err := os.MkdirAll(filepath.Join(real, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "a", "x.txt"), []byte("Name:Xyzzy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "cardsfolder")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, err := corpusAllNames(link)
	if err != nil {
		t.Fatalf("corpusAllNames through a symlinked root: %v", err)
	}
	if string(got) != "Xyzzy\n" {
		t.Errorf("corpusAllNames = %q, want %q", got, "Xyzzy\n")
	}
}
