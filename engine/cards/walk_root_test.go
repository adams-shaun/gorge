package cards

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileScriptsFollowsSymlinkedRoot(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "bear.txt"), []byte("Name:Walk Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "cardsfolder")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cs, _, err := compileScripts(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("got %d cards, want 1", len(cs))
	}
	if want := filepath.Join(link, "bear.txt"); cs[0].Path != want {
		t.Fatalf("Path = %q, want the caller's spelling %q", cs[0].Path, want)
	}
}
