package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
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
// and `git rev-parse --show-toplevel` fails. Were CorpusRegistry to run its
// child git with the inherited environment, the corpus would go missing under
// a pre-commit hook and every corpus test in the package would silently stop
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

func writeCorpus(t *testing.T, dir, lock string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".cards", "cardsfolder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if lock != "" {
		if err := os.WriteFile(filepath.Join(dir, ".cards", "cards.lock"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPickCorpusPrefersTheWorktreesOwn(t *testing.T) {
	root := t.TempDir()
	main, wt := filepath.Join(root, "main"), filepath.Join(root, "wt")
	writeCorpus(t, main, "old")
	// The main checkout's is used while the worktree has none.
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := pickCorpus(filepath.Join(main, ".git"), wt), filepath.Join(main, ".cards"); got != want {
		t.Fatalf("got %s, want main's %s", got, want)
	}
	// A symlinked own .cards counts and wins.
	real := filepath.Join(root, "real")
	writeCorpus(t, real, "new")
	if err := os.Symlink(filepath.Join(real, ".cards"), filepath.Join(wt, ".cards")); err != nil {
		t.Fatal(err)
	}
	if got, want := pickCorpus(filepath.Join(main, ".git"), wt), filepath.Join(wt, ".cards"); got != want {
		t.Fatalf("got %s, want own %s", got, want)
	}
}

func TestPickCorpusFreshCloneIsTheOwnPath(t *testing.T) {
	root := t.TempDir()
	if got, want := pickCorpus(filepath.Join(root, ".git"), root), filepath.Join(root, ".cards"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestCheckCorpusPin(t *testing.T) {
	root := t.TempDir()
	mk := filepath.Join(root, "Makefile")
	if err := os.WriteFile(mk, []byte("X ?= 1\nFORGE_REF  ?= abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCorpus(t, filepath.Join(root, "ok"), `{"ref": "abc123", "commit": "abc123"}`)
	writeCorpus(t, filepath.Join(root, "stale"), `{"ref": "def456"}`)
	writeCorpus(t, filepath.Join(root, "nolock"), "")
	if err := checkCorpusPin(filepath.Join(root, "ok", ".cards"), mk); err != nil {
		t.Errorf("matching pin: %v", err)
	}
	err := checkCorpusPin(filepath.Join(root, "stale", ".cards"), mk)
	if err == nil || !strings.Contains(err.Error(), "abc123") || !strings.Contains(err.Error(), "make link-cards") {
		t.Errorf("stale pin: got %v, want an error naming FORGE_REF and the fix", err)
	}
	if err := checkCorpusPin(filepath.Join(root, "nolock", ".cards"), mk); err != nil {
		t.Errorf("no cards.lock: %v", err)
	}
	if err := checkCorpusPin(filepath.Join(root, "absent", ".cards"), mk); err != nil {
		t.Errorf("no corpus: %v", err)
	}
}

func TestGorgeCardsDirWins(t *testing.T) {
	t.Setenv("GORGE_CARDS_DIR", "/nonexistent/corpus")
	if got, ok := SharedCorpusDir("."); !ok || got != "/nonexistent/corpus" {
		t.Fatalf("got %q %v", got, ok)
	}
}

// A lock that names the right ref but not the scripts beside it fails the
// corpus check once, with the fix: the drift the pin check cannot see.
func TestVerifyCorpusContentRejectsDrift(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good", ".cards")
	writeCorpus(t, filepath.Join(root, "good"), `{"ref":"abc123","files":2}`)
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(good, "cardsfolder", n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyCorpusContent(good); err != nil {
		t.Fatalf("matching corpus: %v", err)
	}
	extra := filepath.Join(root, "extra", ".cards")
	writeCorpus(t, filepath.Join(root, "extra"), `{"ref":"abc123","files":2}`)
	for _, n := range []string{"a.txt", "b.txt", "stale.txt"} {
		if err := os.WriteFile(filepath.Join(extra, "cardsfolder", n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := verifyCorpusContent(extra)
	if err == nil || !strings.Contains(err.Error(), "forgec verify") || !strings.Contains(err.Error(), "--delete") {
		t.Fatalf("stale file under a matching lock: got %v, want an error with the fix", err)
	}
	if err := verifyCorpusContent(filepath.Join(root, "absent", ".cards")); err != nil {
		t.Fatalf("no corpus: %v", err)
	}
}
