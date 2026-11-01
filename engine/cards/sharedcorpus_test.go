package cards

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sharedCorpusDir is testutil.SharedCorpusDir for this package's own tests
// (testutil imports cards, so cards cannot import it). It resolves the
// .cards corpus for code running inside the repository rooted at (or
// containing) dir, preferring the MAIN checkout's copy over the worktree's
// own path.
//
// Every task worktree's .cards is a symlink to the main checkout's
// (linked by hand, or by a companion module's script), so the
// bytes are identical either way. The
// path is not: `go test` keys a cached result on the absolute names of the
// files the test opened, so a corpus opened as <worktree>/.cards/... misses
// the shared GOCACHE in every other worktree and a 2-minute corpus test is
// re-run once per seat. Opening it through the git common dir's checkout
// gives every worktree the same names, and with -trimpath the same test
// binary. Go hashes only files inside the module root, and those by
// absolute name, so a test that also reads testdata still misses across
// worktrees; but re-runs in one worktree now hit, which they never did while
// the stat'd .cards mtime moved (see below). The corpus, being outside a
// worktree's root, is not hashed at all there: `make fetch-cards` clears the
// test cache when the pin moves.
//
// The common-dir path is only taken when it holds a .cards; otherwise (a
// bare or out-of-tree common dir, a clone with no main-checkout corpus) it
// falls back to the worktree's own toplevel. GORGE_CARDS_DIR overrides both.
// ok is false only when git cannot resolve the repository at all.
func sharedCorpusDir(dir string) (corpus string, ok bool) {
	if d := os.Getenv("GORGE_CARDS_DIR"); d != "" {
		return d, true
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute",
		"--git-common-dir", "--show-toplevel")
	cmd.Env = GitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", false
	}
	common, top := lines[0], lines[1]
	if filepath.Base(common) == ".git" {
		shared := filepath.Join(filepath.Dir(common), ".cards")
		// Probe cardsfolder, never .cards itself: every new ir-<fingerprint>
		// cache file bumps the directory's mtime, and a stat'd mtime is part
		// of the test-cache key, so stat'ing .cards un-caches every corpus
		// test on the box whenever any branch compiles the corpus.
		if _, err := os.Stat(filepath.Join(shared, "cardsfolder")); err == nil {
			return shared, true
		}
	}
	return filepath.Join(top, ".cards"), true
}
