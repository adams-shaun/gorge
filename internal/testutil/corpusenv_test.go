package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

func TestCorpusRegistryCacheSharesConcurrentLoad(t *testing.T) {
	var cache corpusRegistryCache
	var calls atomic.Int32
	want := cards.NewRegistry()
	start := make(chan struct{})
	got := make(chan corpusRegistryResult, 16)
	for range 16 {
		go func() {
			<-start
			got <- cache.get(func() corpusRegistryResult {
				calls.Add(1)
				return corpusRegistryResult{reg: want}
			})
		}()
	}
	close(start)
	for range 16 {
		if result := <-got; result.reg != want {
			t.Fatalf("registry = %p, want %p", result.reg, want)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

func TestCorpusRegistryCacheSharesFailure(t *testing.T) {
	var cache corpusRegistryCache
	var calls atomic.Int32
	want := errors.New("load failed")
	load := func() corpusRegistryResult {
		calls.Add(1)
		return corpusRegistryResult{err: want}
	}
	for range 2 {
		if result := cache.get(load); !errors.Is(result.err, want) {
			t.Fatalf("load error = %v, want %v", result.err, want)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

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

	// Exercise the uncached loader directly: another test may already have
	// populated the package-wide CorpusRegistry cache before this test runs.
	// The corpus presence check above means missing here is itself a failure.
	result := loadRepoCorpusRegistry()
	if result.missing {
		t.Fatal("corpus loader reported the existing .cards directory missing")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	reg := result.reg
	if reg == nil {
		t.Fatal("CorpusRegistry returned nil under an exported GIT_DIR")
	}
	if _, ok := reg.Lookup("Lightning Bolt"); !ok {
		t.Fatal("corpus opened under an exported GIT_DIR but does not contain Lightning Bolt; the registry is not the real corpus")
	}
}
