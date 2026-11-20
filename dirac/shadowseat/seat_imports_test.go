package shadowseat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// allowedImports are the module packages the seat may import; the standard
// library is allowed but reflect and unsafe.
var allowedImports = map[string]bool{
	"github.com/adams-shaun/gorge/dirac":             true,
	"github.com/adams-shaun/gorge/dirac/kop":         true,
	"github.com/adams-shaun/gorge/dirac/shadow":      true,
	"github.com/adams-shaun/gorge/dirac/search":      true,
	"github.com/adams-shaun/gorge/dirac/search/hbot": true,
	"github.com/adams-shaun/gorge/dirac/canon":       true,
	"github.com/adams-shaun/gorge/dirac/cardset":     true,
	"github.com/adams-shaun/gorge/engine/view":       true,
	"github.com/adams-shaun/gorge/engine/decision":   true,
	"github.com/adams-shaun/gorge/engine/seat":       true,
	"github.com/adams-shaun/gorge/engine/cards":      true,
}

func TestShadowSeatImports(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		n++
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			std := !strings.Contains(strings.SplitN(path, "/", 2)[0], ".")
			switch {
			case std && (path == "reflect" || path == "unsafe"):
				t.Errorf("%s imports %s", f, path)
			case std, allowedImports[path]:
			default:
				t.Errorf("%s imports %s, which the seat may not", f, path)
			}
		}
		ast.Inspect(af, func(nd ast.Node) bool {
			if id, ok := nd.(*ast.Ident); ok && id.Name == "Clairvoyant" {
				t.Errorf("%s: identifier Clairvoyant at %v: the seat never sets it", f, fset.Position(id.Pos()))
			}
			return true
		})
	}
	if n < 3 {
		t.Fatalf("parsed %d non-test files", n)
	}
}
