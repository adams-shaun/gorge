package codeshape

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Corpus-lookup ratchet (test-cacheability). `go test` keys a cached result on
// every file and directory a test binary stats or opens, and on the absolute
// name of each. A test that stat's the .cards DIRECTORY itself depends on its
// mtime, which moves whenever any branch writes a new ir-<fingerprint> cache
// file, so it never caches; a test that opens the corpus by a per-worktree
// relative path misses the shared cache in every other worktree. Corpus tests
// resolve the corpus through testutil.SharedCorpusDir / CorpusDirOrSkip /
// CorpusRegistry (cards: sharedCorpusDir) and probe cards.CorpusDir(dir)
// (cardsfolder), never .cards.
//
// A _test.go file therefore may not (a) name a "../.cards"-style relative
// corpus root as a string literal, nor (b) hold a `.cards"` literal together
// with os.Stat. The patterns are assembled from pieces so this file does not
// match itself.
var (
	corpusRelRootRe = regexp.MustCompile(`"(?:\.\./)+\` + "." + `cards`)
	corpusDirLit    = "." + "cards" + `"`
	statCall        = "os." + "Stat"
)

// corpusStatAllowed lists test files that may legitimately name the corpus
// directory: the resolvers themselves (both are the one place that probes
// where the corpus is) and the test that proves the resolver's fallback.
func corpusStatAllowed(rel string) bool {
	return strings.HasPrefix(rel, "testkit/testutil/") || rel == "engine/cards/sharedcorpus_test.go" ||
		rel == "internal/codeshape/corpus_stat_test.go"
}

// corpusLookupViolations returns the repo-relative _test.go files under root
// that break the rule above, sorted.
//
// The files come from `git ls-files` (the tracked files, as they are on disk),
// never from a walk of root. go's test cache hashes the listing of every
// directory a test opens, with each entry's mtime, so walking root keyed this
// test on .ds4/, which every seat round writes: it re-ran on every precheck and
// was never cached (test-cache-audit 2026-11-14: 0 of 247 runs). git reads the
// directories in its own process, which go does not record. A file that is not
// yet tracked cannot land, and test-smart and the gate run on commits.
func corpusLookupViolations(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--", "*_test.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", root, err)
	}
	var bad []string
	n := 0
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" || skippedTestPath(rel) {
			continue
		}
		n++
		if corpusStatAllowed(rel) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue // deleted in the work tree, not yet in the index
		}
		if err != nil {
			return nil, err
		}
		src := string(b)
		if corpusRelRootRe.MatchString(src) ||
			(strings.Contains(src, corpusDirLit) && strings.Contains(src, statCall)) {
			bad = append(bad, rel)
		}
	}
	if n == 0 {
		return nil, fmt.Errorf("git ls-files in %s listed no _test.go files: not a checkout with its files tracked", root)
	}
	sort.Strings(bad)
	return bad, nil
}

// skippedTestPath: a path under a dot or underscore directory (outside the go
// tool's package walk), testdata or node_modules.
func skippedTestPath(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, d := range parts[:len(parts)-1] {
		if strings.HasPrefix(d, ".") || strings.HasPrefix(d, "_") || d == "testdata" || d == "node_modules" {
			return true
		}
	}
	return false
}

func TestNoTestStatsOrRelativelyOpensTheCorpusDir(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bad, err := corpusLookupViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range bad {
		t.Errorf("%s resolves the corpus privately (a \"../.cards\" literal, or os.Stat beside a .cards literal): "+
			"use testutil.SharedCorpusDir / CorpusDirOrSkip and stat cards.CorpusDir(dir), never .cards itself", f)
	}
}

// TestCorpusLookupRatchetCanFail pins the detector: each forbidden shape is
// flagged, and the conforming shape is not.
func TestCorpusLookupRatchetCanFail(t *testing.T) {
	q := `"`
	files := map[string]string{
		"a/rel_test.go":  "package a\nvar d = " + q + "../" + "." + "cards/cardsfolder" + q + "\n",
		"a/deep_test.go": "package a\nvar d = " + q + "../../" + "." + "cards" + q + "\n",
		"a/stat_test.go": "package a\nfunc f() { _ = filepath.Join(top, " + corpusDirLit + "); " + statCall + "(d) }\n",
		"a/ok_test.go":   "package a\nfunc f() { d := testutil.CorpusDirOrSkip(t); " + statCall + "(cards.CorpusDir(d)) }\n",
		"a/join_test.go": "package a\nvar d = filepath.Join(" + q + ".." + q + ", " + corpusDirLit + ", " + q + "cardsfolder" + q + ")\n",
		// outside the go tool's package walk: never flagged
		".ds4/x/rel_test.go":     "package x\nvar d = " + q + "../" + "." + "cards" + q + "\n",
		"a/testdata/rel_test.go": "package x\nvar d = " + q + "../" + "." + "cards" + q + "\n",
		"_scratch/rel_test.go":   "package x\nvar d = " + q + "../" + "." + "cards" + q + "\n",
	}
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCheckout(t, root)
	got, err := corpusLookupViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/deep_test.go", "a/rel_test.go", "a/stat_test.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}

// gitCheckout makes root a git checkout with every file in it tracked (the
// detector lists files with git ls-files), or skips without git.
func gitCheckout(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, a := range [][]string{{"init", "-q"}, {"add", "-A", "-f"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
}

// TestCorpusLookupNeverListsTheRoot pins the cacheability fix: the detector
// must not open the checkout's root directory (go's test cache would key the
// result on the root listing, and so on .ds4/'s mtime). With the root
// unreadable, a walk fails; the git listing still finds the violation.
func TestCorpusLookupNeverListsTheRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	root := t.TempDir()
	q := `"`
	p := filepath.Join(root, "a", "rel_test.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("package a\nvar d = "+q+"../"+"."+"cards"+q+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCheckout(t, root)
	if err := os.Chmod(root, 0o311); err != nil { // traversable, not listable
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) })
	got, err := corpusLookupViolations(root)
	if err != nil {
		t.Fatalf("the detector needs the root's listing: %v", err)
	}
	if strings.Join(got, ",") != "a/rel_test.go" {
		t.Fatalf("violations = %v, want [a/rel_test.go]", got)
	}
}
