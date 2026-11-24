package effects

import (
	"path/filepath"
	"testing"
)

// corpusWalkRoot resolves a corpus directory before a filepath.WalkDir: WalkDir
// does not follow a symlinked root, and a worktree's cardsfolder is one, so an
// unresolved walk sees no card scripts and a census passes or fails vacuously.
func corpusWalkRoot(t testing.TB, dir string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}
