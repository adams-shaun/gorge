package draft_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// forbiddenImports are the imports banned from the
// pure draft/limited/ packages: no clock, no file or network I/O, no legacy
// math/rand, and none of the engine tiers. limited is the only gorge
// package draft/limited/draft may import.
var forbiddenImports = []string{
	"time",
	"os",
	"io/fs",
	"net",
	"net/http",
	"math/rand",
	"github.com/adams-shaun/gorge/engine/cards",
	"github.com/adams-shaun/gorge/engine/deck",
	"github.com/adams-shaun/gorge/engine/rules",
	"github.com/adams-shaun/gorge/engine/state",
	"github.com/adams-shaun/gorge/engine/events",
	"github.com/adams-shaun/gorge/engine/effects",
	"github.com/adams-shaun/gorge/server/host",
	"github.com/adams-shaun/gorge/engine/view",
	"github.com/adams-shaun/gorge/engine/seat",
	"github.com/adams-shaun/gorge/bots",
}

func TestImportsStayPure(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	if len(files) == 0 {
		t.Fatal("no non-test Go files found: the purity scan scanned nothing")
	}
	sort.Strings(files)
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				if p == bad || strings.HasPrefix(p, bad+"/") {
					t.Errorf("%s imports %q: %s must stay pure", f.Name.Name, p, dir)
				}
			}
		}
	}
}
