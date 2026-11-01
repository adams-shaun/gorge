package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtagentImportsStayClean guards the command's own shape: no wall
// clock (internal/archtest's TestTimeIsImportedOnlyByTheHost refuses any
// new `time` importer outside its allow-list) and no legacy math/rand
// (TestNoLegacyMathRand bans it module-wide; the seeded math/rand/v2 is
// the only rng).
func TestExtagentImportsStayClean(t *testing.T) {
	banned := map[string]string{
		"time":      "the archtest time allow-list is closed; the command needs no clock",
		"math/rand": "use the seeded math/rand/v2",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if why, bad := banned[path]; bad {
				t.Errorf("%s imports %q: %s", name, path, why)
			}
		}
	}
}
