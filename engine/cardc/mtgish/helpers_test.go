package mtgish

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Shared helpers for the front-end tickets (CC-10..14): each adds its own
// fe_*.go and fe_*_test.go and uses these; none edits lower.go or this file.

var (
	corpusOnce  sync.Once
	corpusCards []*Card
	corpusIndex *Index
	corpusErr   error
)

// corpus parses the data once per test binary (it is a 30 MB file) and
// returns every card and the index over them. It skips with the ENV message
// when the data is absent.
func corpus(t *testing.T) ([]*Card, *Index) {
	t.Helper()
	p := DataPath()
	if p == "" {
		t.Skip(SkipMessage)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip(SkipMessage)
	}
	corpusOnce.Do(func() {
		corpusCards, corpusErr = LoadAll(p)
		if corpusErr == nil {
			corpusIndex = NewIndex(corpusCards)
		}
	})
	if corpusErr != nil {
		t.Fatal(corpusErr)
	}
	return corpusCards, corpusIndex
}

// corpusSchema reads the schema, skipping with the ENV message when absent.
func corpusSchema(t *testing.T) Schema {
	t.Helper()
	p := SchemaPath()
	if p == "" {
		t.Skip(SkipMessage)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip(SkipMessage)
	}
	s, err := ReadSchema(p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// resolveSlice resolves the named slices through the index, failing the test
// on any name that does not resolve. The result is deduplicated by card.
func resolveSlice(t *testing.T, slices ...string) []*Card {
	t.Helper()
	_, ix := corpus(t)
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	var out []*Card
	seen := map[*Card]bool{}
	var missing []string
	for _, s := range slices {
		names, err := SliceNames(root, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			c, ok := ix.Lookup(n)
			if !ok {
				missing = append(missing, s+": "+n)
				continue
			}
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d names do not resolve: %s", len(missing), strings.Join(missing, "; "))
	}
	return out
}

// proofSlice is the plan's proof slice: the cards of fdn plus legacy.
func proofSlice(t *testing.T) []*Card {
	t.Helper()
	return resolveSlice(t, "fdn", "legacy")
}

// familyCoverageProblems is pure. For every census key whose enum is in
// enums it reports a key neither registered nor deferred; it reports a
// deferred key that is registered (stale) and a deferred key that no longer
// occurs in the census. Each deferred value is a one-line reason.
func familyCoverageProblems(census []VariantCount, enums []string, deferred map[string]string, registered func(string) bool) []string {
	want := map[string]bool{}
	for _, e := range enums {
		want[e] = true
	}
	inCensus := map[string]bool{}
	var out []string
	for _, vc := range census {
		inCensus[vc.Key] = true
		enum, _, _ := strings.Cut(vc.Key, ".")
		if !want[enum] {
			continue
		}
		_, isDeferred := deferred[vc.Key]
		switch {
		case !registered(vc.Key) && !isDeferred:
			out = append(out, fmt.Sprintf("%s: used by %d cards, neither registered nor deferred", vc.Key, vc.Cards))
		case registered(vc.Key) && isDeferred:
			out = append(out, fmt.Sprintf("%s: deferred (%s) but registered; drop the deferral", vc.Key, deferred[vc.Key]))
		}
	}
	keys := make([]string, 0, len(deferred))
	for k := range deferred {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !inCensus[k] {
			out = append(out, fmt.Sprintf("%s: deferred (%s) but no card in the census uses it", k, deferred[k]))
		}
	}
	sort.Strings(out)
	return out
}

// checkFamilyCoverage runs the census over the proof slice and reports each
// problem familyCoverageProblems finds for enums.
func checkFamilyCoverage(t *testing.T, enums []string, deferred map[string]string) {
	t.Helper()
	for _, p := range familyCoverageProblems(Census(proofSlice(t)), enums, deferred, Registered) {
		t.Error(p)
	}
}

// withStubs overlays test-only lowerings on the registry for one test and
// restores the overlay in t.Cleanup. lower consults the overlay first. A test
// that uses it must not call t.Parallel.
func withStubs(t *testing.T, stubs map[string]lowerFunc) {
	t.Helper()
	prev := overlay
	next := make(map[string]lowerFunc, len(prev)+len(stubs))
	for k, f := range prev {
		next[k] = f
	}
	for k, f := range stubs {
		next[k] = f
	}
	overlay = next
	t.Cleanup(func() { overlay = prev })
}
