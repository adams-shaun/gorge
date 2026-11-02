package kop

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// extConsts type-checks the package's non-test files and returns every
// constant of type ExtOp with its value.
func extConsts(t *testing.T) map[string]int64 {
	return typedConsts(t, "kop.ExtOp", "NumExtOps")
}

// opConsts is every constant of type Op below OpHighEnd except the range
// markers (OpCount, OpHighFirst, OpHighEnd).
func opConsts(t *testing.T) map[string]int64 {
	return typedConsts(t, "kop.Op", "OpCount", "OpHighFirst", "OpHighEnd")
}

// sourceImporter type-checks from source. A -trimpath test binary knows no GOROOT unless its
// environment gives it one, and go/build can't find a package without it.
func sourceImporter(t *testing.T, fset *token.FileSet) types.Importer {
	t.Helper()
	if build.Default.GOROOT == "" {
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			t.Fatal(err)
		}
		build.Default.GOROOT = strings.TrimSpace(string(out))
	}
	return importer.ForCompiler(fset, "source", nil)
}

func typedConsts(t *testing.T, typ string, skip ...string) map[string]int64 {
	t.Helper()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
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
		files = append(files, f)
	}
	conf := types.Config{Importer: sourceImporter(t, fset)}
	pkg, err := conf.Check("kop", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, n := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(n).(*types.Const)
		if !ok || c.Type().String() != typ {
			continue
		}
		skipped := false
		for _, k := range skip {
			skipped = skipped || n == k
		}
		if skipped {
			continue
		}
		v, _ := constInt(c)
		out[n] = v
	}
	return out
}

func constInt(c *types.Const) (int64, bool) {
	var v int64
	_, err := fmt.Sscan(c.Val().ExactString(), &v)
	return v, err == nil
}

// TestRegistryNamesComplete: every Ext sub-op constant registers a name
// (extOpNames[ExtX] = "X" in an init beside its implementation), and no
// registration lacks a constant.
func TestRegistryNamesComplete(t *testing.T) {
	cs := extConsts(t)
	byVal := map[int64]bool{}
	for n, v := range cs {
		byVal[v] = true
		if v < 0 || v >= int64(NumExtOps) {
			t.Errorf("%s = %d is outside NumExtOps %d", n, v, NumExtOps)
			continue
		}
		if extOpNames[v] == "" {
			t.Errorf("%s = %d has no name: add extOpNames[%s] = %q in the sub-op's own file", n, v, n, strings.TrimPrefix(n, "Ext"))
		}
	}
	for v, n := range extOpNames {
		if n != "" && !byVal[int64(v)] {
			t.Errorf("extOpNames[%d] = %q names no constant", v, n)
		}
	}
}

// TestEnumValuesUnique: no two Ext sub-op constants share a value.
func TestEnumValuesUnique(t *testing.T) {
	seen := map[int64]string{}
	for n, v := range extConsts(t) {
		if o, dup := seen[v]; dup {
			t.Errorf("%s and %s share value %d", o, n, v)
		}
		seen[v] = n
	}
}

// TestRegistryOpNamesComplete: every Op constant has a name (the frozen
// literal, or opNames[OpX] = "X" in an init in the op's own file), the iota
// list ends exactly at OpCount, and Known agrees.
func TestRegistryOpNamesComplete(t *testing.T) {
	cs := opConsts(t)
	byVal := map[int64]bool{}
	for n, v := range cs {
		byVal[v] = true
		if v < 0 || v >= int64(OpHighEnd) {
			t.Errorf("%s = %d is out of range", n, v)
			continue
		}
		if o := Op(v); o.String() == "Op(?)" || !o.Known() {
			t.Errorf("%s = %d has no name: add opNames[%s] = %q in the op's own file", n, v, n, strings.TrimPrefix(n, "Op"))
		}
	}
	for v := Op(0); v < OpHighEnd; v++ {
		if v.Known() && !byVal[int64(v)] {
			t.Errorf("op %d (%s) is known but names no constant", v, v)
		}
	}
	if int64(OpBlight)+1 != int64(OpCount) {
		t.Errorf("the iota op list is frozen at OpCount=%d but OpBlight+1=%d: new ops are explicit constants in their own file", OpCount, OpBlight+1)
	}
}

// TestOpValuesUnique: no two Op constants share a value.
func TestOpValuesUnique(t *testing.T) {
	seen := map[int64]string{}
	for n, v := range opConsts(t) {
		if o, dup := seen[v]; dup {
			t.Errorf("%s and %s share value %d", o, n, v)
		}
		seen[v] = n
	}
}
