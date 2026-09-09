package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCorpusRegistryResolvesUnderAnExportedGitDir pins the gitiso hazard on the
// corpus locator itself.
//
// Git exports GIT_DIR to every hook it runs, commonly as the relative path
// ".git". A test binary's working directory is its own package directory, not
// the repo root, so a relative GIT_DIR resolves to a path that does not exist
// and `git rev-parse --show-toplevel` fails. Before the fix, CorpusRegistry ran
// its child git with the inherited environment, so under a pre-commit hook the
// corpus went missing and every corpus test in the package silently stopped
// exercising the corpus -- a fast, green, entirely fictional run.
//
// The test arms exactly that environment and asserts the corpus is still found.
func TestCorpusRegistryResolvesUnderAnExportedGitDir(t *testing.T) {
	// Reproduce a test binary's real situation: cwd is a package directory
	// below the repo root, and GIT_DIR is the relative ".git" a hook exports.
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "..", "..", ".cards")); err != nil {
		t.Skip("no .cards/ corpus present -- nothing to prove")
	}

	t.Setenv("GIT_DIR", ".git")
	t.Setenv("GIT_INDEX_FILE", ".git/index")

	// CorpusRegistry t.Fatals if it cannot resolve the root and t.Skips only
	// when there is genuinely no corpus. A skip here would itself be the bug
	// coming back, so the corpus presence check above is deliberately made
	// before the environment is armed.
	reg := CorpusRegistry(t)
	if reg == nil {
		t.Fatal("CorpusRegistry returned nil under an exported GIT_DIR")
	}
	if _, ok := reg.Lookup("Lightning Bolt"); !ok {
		t.Fatal("corpus opened under an exported GIT_DIR but does not contain Lightning Bolt; the registry is not the real corpus")
	}
}
