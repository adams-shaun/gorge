package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// SharedCorpusDir resolves the .cards corpus for code running inside the
// repository rooted at (or containing) dir. GORGE_CARDS_DIR wins; otherwise
// the worktree's own .cards (a directory or a symlink) when present, else the
// main checkout's (see pickCorpus). ok is false only when git cannot resolve
// the repository at all.
//
// The worktree's own copy wins over the main checkout's: a Forge-corpus bump
// branch links its own .cards at the new pin, and the main checkout's still
// holds the old one, which worktree gates tested silently (11-23).
// Whichever corpus is chosen, a cards.lock that does not name the Makefile's
// FORGE_REF panics with the fix, so the test fails loudly instead of running
// against the wrong corpus. No corpus at all is unchanged: callers skip.
//
// A corpus opened through a worktree's own path misses the shared GOCACHE of
// other worktrees; that is the price of testing the right corpus.
func SharedCorpusDir(dir string) (corpus string, ok bool) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute",
		"--git-common-dir", "--show-toplevel")
	cmd.Env = cards.GitEnv()
	env := os.Getenv("GORGE_CARDS_DIR")
	out, err := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if err != nil || len(lines) != 2 {
		// Git cannot place the repository: an explicit corpus still works,
		// unchecked (there is no Makefile to read its pin from).
		return env, env != ""
	}
	corpus = env
	if corpus == "" {
		corpus = pickCorpus(lines[0], lines[1])
	}
	if err := checkCorpusPin(corpus, filepath.Join(lines[1], "Makefile")); err != nil {
		// A stale corpus must never be tested silently: a test that gets here
		// is a FAIL (not a skip, which the make targets would class ENV).
		panic(err.Error())
	}
	return corpus, true
}

// pickCorpus chooses the corpus directory of the checkout whose git common dir
// and toplevel are given: the worktree's own .cards (a directory or a symlink)
// when it holds a cardsfolder, else the main checkout's, else the toplevel's
// path (which may not exist: a fresh clone, where callers skip).
//
// Probe cardsfolder, never .cards itself: every new ir-<fingerprint> cache
// file bumps the directory's mtime, and a stat'd mtime is part of the
// test-cache key, so stat'ing .cards un-caches every corpus test on the box
// whenever any branch compiles the corpus.
func pickCorpus(common, top string) string {
	own := filepath.Join(top, ".cards")
	if _, err := os.Stat(filepath.Join(own, "cardsfolder")); err == nil {
		return own
	}
	if filepath.Base(common) == ".git" {
		shared := filepath.Join(filepath.Dir(common), ".cards")
		if _, err := os.Stat(filepath.Join(shared, "cardsfolder")); err == nil {
			return shared
		}
	}
	return own
}

// checkCorpusPin fails when corpus holds a cards.lock that does not name the
// FORGE_REF of makefile. No cards.lock or no Makefile is not an error: there
// is nothing to compare (a fresh clone has no corpus and its tests skip).
func checkCorpusPin(corpus, makefile string) error {
	lock, err := os.ReadFile(filepath.Join(corpus, "cards.lock"))
	if err != nil {
		return nil
	}
	mk, err := os.ReadFile(makefile)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(mk), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[0] == "FORGE_REF" && f[1] == "?=" {
			if !strings.Contains(string(lock), f[2]) {
				return fmt.Errorf("testutil: corpus %s is not this tree's pin: its cards.lock lacks FORGE_REF %s (Makefile). "+
					"It is another branch's corpus: run `make link-cards` (or `make fetch-cards compile-cards`), or set GORGE_CARDS_DIR to a corpus at the pin", corpus, f[2])
			}
			return nil
		}
	}
	return nil
}

// CorpusDirOrSkip is SharedCorpusDir for a test that needs the corpus
// DIRECTORY (a CLI's -dir, a cardsfolder walk) rather than a registry. It
// Skips when the corpus was never fetched, probing cardsfolder and never
// .cards itself (a stat'd .cards mtime un-caches the test; see
// SharedCorpusDir), and fails when git cannot resolve the repository.
func CorpusDirOrSkip(t testing.TB) string {
	t.Helper()
	dir, ok := SharedCorpusDir(".")
	if !ok {
		t.Fatalf("testutil: could not resolve git repo root")
	}
	if _, err := os.Stat(cards.CorpusDir(dir)); err != nil {
		t.Skip("testutil: no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	return dir
}
