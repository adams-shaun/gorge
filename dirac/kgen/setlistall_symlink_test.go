package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusAllNamesFollowsSymlinkedRoot(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "Name:Symlinked Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"
	if err := os.WriteFile(filepath.Join(real, "symlinked_bear.txt"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "cardsfolder")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("precondition: root must be a symlink: %v", err)
	}
	got, err := corpusAllNames(link)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Symlinked Bear\n" {
		t.Fatalf("got %q", got)
	}
}
