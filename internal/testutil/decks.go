package testutil

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
)

// decksFS embeds the project's own 12 Legacy archetype deck lists -- a bare
// {name, count} card list per deck, authored for this repo and Apache-2.0
// like the rest of it. Embedding these is not the licensing hazard
// cards/boundary_test.go guards against: that guard fires on a Forge
// script's own Name:/Oracle: shape (GPL-3.0), which a JSON name-and-count
// list never carries, no matter how it is packaged (Ruling P11).
//
//go:embed decks/*.json
var decksFS embed.FS

const decksDir = "decks"

// RepoDeckNames lists the embedded decks by file stem (e.g.
// "mono-red-goblins"), sorted. Sorting matters: callers index into this
// slice to assign decks to seats (Task 26's acceptance test does exactly
// that for 2/4/6/8 seats), and embed.FS.ReadDir's own order, while already
// alphabetical in the standard library, is not a documented guarantee this
// package should lean on.
func RepoDeckNames() []string {
	entries, err := decksFS.ReadDir(decksDir)
	if err != nil {
		// The embed is compiled into the binary at build time; a failure
		// here means decks/ vanished from the source tree, not something a
		// caller can recover from at runtime.
		panic("testutil: decks/*.json embed unreadable: " + err.Error())
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".json"); ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// LoadRepoDeck reads decks/<name>.json and resolves it through the deck
// package (deck.Parse then File.Resolve, which runs cards.NormalizeName so
// Scryfall-shaped catalogue names and Forge script names fold to the same
// key) into a flat, count-many-times-repeated slice of *cards.Card pointers
// -- the shape rules.Config.Decks wants. It is the non-testing.TB variant
// mtgsim uses directly; RepoDeck below is the t.Fatalf-on-error wrapper for
// tests (Ruling P11).
func LoadRepoDeck(r *cards.Registry, name string) ([]*cards.Card, error) {
	// embed.FS paths are always slash-separated regardless of host OS --
	// filepath.Join would emit "decks\name.json" on Windows and the read
	// would fail there. path.Join is the correct call for an embed.FS path.
	raw, err := decksFS.ReadFile(path.Join(decksDir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("testutil: deck %q: %w", name, err)
	}
	f, err := deck.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("testutil: deck %q: %w", name, err)
	}
	cs, err := f.Resolve(r)
	if err != nil {
		return nil, fmt.Errorf("testutil: %w", err)
	}
	return cs, nil
}

// RepoDeck is LoadRepoDeck for tests: a missing embedded deck file or a card
// name the registry cannot resolve is a fixture problem the test has no
// useful way to continue past, so it fails immediately via t.Fatalf rather
// than handing the caller an error to (potentially inconsistently) check.
func RepoDeck(t testing.TB, r *cards.Registry, name string) []*cards.Card {
	t.Helper()
	deck, err := LoadRepoDeck(r, name)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return deck
}

// OpenCorpusRegistry opens dir's compiled corpus, compiling it fresh from
// dir/cardsfolder when the cache is missing or stale. The logic now lives
// in cards.OpenCorpus; this wrapper stays so CorpusRegistry below (and any
// other existing caller) keeps working unchanged.
func OpenCorpusRegistry(dir string) (*cards.Registry, error) {
	return cards.OpenCorpus(dir)
}

// CorpusRegistry finds the repo root the way cards/boundary_test.go does --
// `git rev-parse --show-toplevel`, not a hard-coded relative path, which is
// exactly what silently broke that file's own license-boundary test when
// this package was transplanted to a new module root (Ruling P11) -- and
// opens its .cards/ corpus. A checkout with no .cards/ directory at all
// (nothing ever fetched) is not a test failure: it Skips, so `go test
// ./...` still passes on a clean clone with no network access. Once .cards/
// exists, though, a corpus that fails to open is a real problem worth
// stopping on, not skipping past.
func CorpusRegistry(t testing.TB) *cards.Registry {
	t.Helper()
	out, err := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("testutil: could not resolve git repo root: %v", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("testutil: no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	r, err := OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatalf("testutil: could not open corpus at %s: %v", dir, err)
	}
	return r
}
