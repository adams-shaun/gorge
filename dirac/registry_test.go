package dirac

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stallConsts returns every Stall* constant declared in the package's
// non-test files. A value that is not an integer literal (NumStall aside) is
// a failure: registrations must stay checkable.
func stallConsts(t *testing.T) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		src, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, n, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if !strings.HasPrefix(id.Name, "Stall") || id.Name == "NumStall" || id.Name == "StallName" || id.Name == "StallReason" || id.Name == "StallCapacity" || id.Name == "StallUnmodelled" {
						continue
					}
					if i >= len(vs.Values) {
						t.Errorf("%s: %s has no literal value", n, id.Name)
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.INT {
						t.Errorf("%s: %s is not an integer literal", n, id.Name)
						continue
					}
					v, err := strconv.Atoi(lit.Value)
					if err != nil {
						t.Errorf("%s: %s: %v", n, id.Name, err)
						continue
					}
					out[id.Name] = v
				}
			}
		}
	}
	return out
}

// TestRegistryNamesComplete: every stall reason constant registers a name
// (stallNames[StallX] = "x" in an init beside its use), and no registration
// lacks a constant.
func TestRegistryNamesComplete(t *testing.T) {
	cs := stallConsts(t)
	byVal := map[int]bool{}
	for n, v := range cs {
		byVal[v] = true
		if v < 0 || v >= NumStall {
			t.Errorf("%s = %d is outside NumStall %d", n, v, NumStall)
			continue
		}
		if v != StallNone && stallNames[v] == "" {
			t.Errorf("%s = %d has no name: add stallNames[%s] = %q in its own file", n, v, n, strings.ToLower(strings.TrimPrefix(n, "Stall")))
		}
	}
	for v, n := range stallNames {
		if n != "" && !byVal[v] {
			t.Errorf("stallNames[%d] = %q names no constant", v, n)
		}
	}
}

// TestEnumValuesUnique: no two stall reason constants share a value.
func TestEnumValuesUnique(t *testing.T) {
	seen := map[int]string{}
	for n, v := range stallConsts(t) {
		if o, dup := seen[v]; dup {
			t.Errorf("%s and %s share value %d", o, n, v)
		}
		seen[v] = n
	}
}
