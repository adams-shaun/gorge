package extern

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExternImportsStayClean guards the package's own shape: no wall clock
// (internal/archtest's TestTimeIsImportedOnlyByTheHost refuses any new
// `time` importer outside its allow-list, and flushing is transition-driven
// here, never timer-driven) and no legacy math/rand (the seeded
// math/rand/v2 is the only rng; this package needs none at all). The
// transports' os/exec and net reach time only transitively, which the
// archtest's direct-import rule allows.
func TestExternImportsStayClean(t *testing.T) {
	banned := map[string]string{
		"time":      "the archtest time allow-list is closed; the hub flushes on transitions, never on a timer",
		"math/rand": "use the seeded math/rand/v2",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || name == "imports_test.go" {
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
