package codeshape

import (
	"io/fs"
	"os"
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
func corpusLookupViolations(root string) ([]string, error) {
	var bad []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Dot and underscore dirs are outside the go tool's package
			// walk, and a local checkout may hold untracked copies in them.
			name := d.Name()
			if p != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if corpusStatAllowed(rel) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		src := string(b)
		if corpusRelRootRe.MatchString(src) ||
			(strings.Contains(src, corpusDirLit) && strings.Contains(src, statCall)) {
			bad = append(bad, rel)
		}
		return nil
	})
	sort.Strings(bad)
	return bad, err
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
	got, err := corpusLookupViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/deep_test.go", "a/rel_test.go", "a/stat_test.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}
