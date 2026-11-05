package main

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestDeckpoolImportsStayClean guards the tool's shape: no wall clock and no
// legacy math/rand, so two runs with the same flags write the same bytes.
func TestDeckpoolImportsStayClean(t *testing.T) {
	banned := map[string]string{
		"time":      "the archtest time allow-list is closed; deckpool needs no clock",
		"math/rand": "no ambient rng: pods are seeded by --seed",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	seen := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		seen++
		f, err := parser.ParseFile(fset, e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if why, bad := banned[path]; bad {
				t.Errorf("%s imports %q: %s", e.Name(), path, why)
			}
		}
	}
	if seen < 3 {
		t.Fatalf("only %d go files seen; the walk is vacuous", seen)
	}
}
