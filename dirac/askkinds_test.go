package dirac

import (
	"bytes"
	"fmt"
	"go/ast"
	gobuild "go/build"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestAskKindsUnchanged holds every resolution ask kind (a package-level
// uint8 constant named ra<Upper>...) to one record of its value: a
// dirac/manifest/ask entry, or a row of testdata/asks.tsv. The file lists the
// hand constants that existed when ask kinds became a manifest family and is
// frozen: kmanifest check refuses any row it would add, so a new constant is
// declared as dirac/manifest/ask/<name> with value: next, which takes the
// lowest free value of the family's band. Every kind has a value no other
// has, since resCtx.ask is compared against them in one switch.
// DIRAC_ASKS_REGEN=1 writes the file (the migration only; make test-one does
// not pass it on).
func TestAskKindsUnchanged(t *testing.T) {
	t.Parallel()
	got := askConsts(t)
	tr, err := manifest.Load(os.DirFS(".."))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]int64{}
	for _, e := range tr.Entries {
		if e.Family == "ask" {
			entries[e.Name] = e.Value
		}
	}
	if os.Getenv("DIRAC_ASKS_REGEN") != "" {
		var names []string
		for n := range got {
			if _, ok := entries[n]; !ok {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		var b bytes.Buffer
		for _, n := range names {
			fmt.Fprintf(&b, "%s\t%d\n", n, got[n])
		}
		if err := os.WriteFile("../"+manifest.AsksPath, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	rows := map[string]int64{}
	for _, a := range tr.Asks {
		rows[a.Name] = a.Value
	}
	if len(rows) < 100 {
		t.Fatalf("%s has %d rows", manifest.AsksPath, len(rows))
	}
	byValue := map[int64]string{}
	var names []string
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		v := got[n]
		if o, dup := byValue[v]; dup {
			t.Errorf("ask kinds %s and %s are both %d", o, n, v)
		}
		byValue[v] = n
		ev, isEntry := entries[n]
		rv, isRow := rows[n]
		switch {
		case isEntry && ev != v:
			t.Errorf("%s is %d, but dirac/manifest/ask/%s says %d", n, v, n, ev)
		case isRow && rv != v:
			t.Errorf("%s is %d, but %s says %d: a value never changes", n, v, manifest.AsksPath, rv)
		case !isEntry && !isRow:
			t.Errorf("%s is %d and has no record: declare dirac/manifest/ask/%s (value: next)", n, v, n)
		}
	}
	for n, v := range rows {
		if _, ok := got[n]; !ok {
			t.Errorf("%s: row %s %d has no constant in package dirac", manifest.AsksPath, n, v)
		}
	}
	for n := range entries {
		if _, ok := got[n]; !ok {
			t.Errorf("dirac/manifest/ask/%s: no constant in package dirac (run kmanifest gen)", n)
		}
	}
}

// askConsts is every ra<Upper>... uint8 constant of package dirac's non-test
// source (the generated file included), type-checked rather than listed.
func askConsts(t *testing.T) map[string]int64 {
	t.Helper()
	if gobuild.Default.GOROOT == "" {
		// A -trimpath test binary knows no GOROOT unless its environment gives it one.
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			t.Fatal(err)
		}
		gobuild.Default.GOROOT = strings.TrimSpace(string(out))
	}
	bp, err := gobuild.Default.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, n := range bp.GoFiles {
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("dirac", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok || len(name) < 3 || !strings.HasPrefix(name, "ra") || name[2] < 'A' || name[2] > 'Z' {
			continue
		}
		if b, ok := c.Type().(*types.Basic); !ok || b.Kind() != types.Uint8 {
			continue
		}
		v, ok := constant.Int64Val(c.Val())
		if !ok {
			t.Fatalf("%s: %v is not an int64", name, c.Val())
		}
		out[name] = v
	}
	return out
}
