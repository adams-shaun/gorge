package kop

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/manifest"
)

const ordinalsFile = "testdata/ordinals.tsv"

// TestKopOrdinalsUnchanged holds every constant of this package to the name, type
// and value in testdata/ordinals.tsv, in both directions: card tables and trained
// models store these values, so a constant keeps its value wherever it is declared.
// The file holds the hand constants only: a constant that is a dirac/manifest
// entry's own (kmanifest gen writes it) has its row derived from the entry
// (manifest.Tree.EntryOrdinals), so a new entry edits no shared file. A new
// hand constant adds its row with
// KOP_ORDINALS_REGEN=1 go test -run TestKopOrdinalsUnchanged ./dirac/kop.
func TestKopOrdinalsUnchanged(t *testing.T) {
	got := constantRows(t)
	derived := derivedRows(t)
	if os.Getenv("KOP_ORDINALS_REGEN") != "" {
		var b bytes.Buffer
		for _, row := range got {
			if !derived[row] {
				fmt.Fprintln(&b, row)
			}
		}
		if err := os.WriteFile(ordinalsFile, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(ordinalsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(want) < 1000 {
		t.Fatalf("%s has %d rows", ordinalsFile, len(want))
	}
	wanted := map[string]bool{}
	for _, row := range want {
		wanted[row] = true
	}
	for row := range derived {
		wanted[row] = true
	}
	want = append(want, sortedRows(derived)...)
	present := map[string]bool{}
	for _, row := range got {
		present[row] = true
		if !wanted[row] {
			t.Errorf("not in %s: %s", ordinalsFile, row)
		}
	}
	for _, row := range want {
		if !present[row] {
			t.Errorf("in %s, not in the package: %s", ordinalsFile, row)
		}
	}
}

// constantRows is every package-level constant here as name, type and exact value,
// tab-separated and sorted, from the source rather than a list that could go stale.
func constantRows(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, f := range parsed["kop"].Files {
		files = append(files, f)
	}
	if build.Default.GOROOT == "" {
		// A -trimpath test binary knows no GOROOT unless its environment gives it one.
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			t.Fatal(err)
		}
		build.Default.GOROOT = strings.TrimSpace(string(out))
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("kop", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		if c, ok := scope.Lookup(name).(*types.Const); ok {
			rows = append(rows, name+"\t"+types.TypeString(c.Type(), types.RelativeTo(pkg))+"\t"+c.Val().ExactString())
		}
	}
	sort.Strings(rows)
	return rows
}

// derivedRows is the rows of the manifest's kop entries (name, type, value,
// tab-separated), the constants kmanifest gen writes.
func derivedRows(t *testing.T) map[string]bool {
	t.Helper()
	tr, err := manifest.Load(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range tr.EntryOrdinals() {
		out[r.Name+"\t"+r.Type+"\t"+r.Value] = true
	}
	return out
}

func sortedRows(m map[string]bool) []string {
	var out []string
	for r := range m {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
