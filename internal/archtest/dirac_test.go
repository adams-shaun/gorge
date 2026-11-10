package archtest

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const diracTree = module + "/dirac"

// diracCore lists the packages dirac's rules run on. They hold a game with no
// gorge package underneath them; only the oracle and kgen, which compare against
// gorge and compile its IR, may reach the engine.
var diracCore = []string{"", "/kop", "/cardset", "/vanilla", "/feat", "/search"}

func inDiracCore(path string) bool {
	for _, c := range diracCore {
		p := diracTree + c
		if path == p || (c != "" && strings.HasPrefix(path, p+"/")) {
			return true
		}
	}
	return false
}

// diracCoreEdges names each dependency of a core package on the module outside
// dirac/. The module root is allowed: it holds only the embedded table sources.
func diracCoreEdges(pkgs map[string]pkg) []string {
	var out []string
	for path, p := range pkgs {
		if !inDiracCore(path) {
			continue
		}
		for dep := range p.deps {
			if strings.HasPrefix(dep, module+"/") && dep != diracTree && !strings.HasPrefix(dep, diracTree+"/") {
				out = append(out, path+" -> "+dep)
			}
		}
	}
	sort.Strings(out)
	return out
}

// requireDirac fails rather than letting a rename leave these rows passing over nothing.
func requireDirac(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join("..", "..", "dirac", "doc.go")); err != nil {
		t.Fatal(err)
	}
}

func TestDiracCoreImportsNoGorge(t *testing.T) {
	requireDirac(t)
	pkgs := packages(t)
	if _, ok := pkgs[diracTree]; !ok {
		t.Fatalf("%s is not in go list's output: the test would run vacuously", diracTree)
	}
	if bad := diracCoreEdges(pkgs); len(bad) > 0 {
		t.Errorf("dirac's core depends on gorge:\n\t%s", strings.Join(bad, "\n\t"))
	}
}

func TestDiracCoreEdgesSynthetic(t *testing.T) {
	got := diracCoreEdges(map[string]pkg{
		diracTree + "/kop":         {deps: set(module)},
		diracTree + "/search/hbot": {deps: set(diracTree + " " + module + "/engine/state")},
		diracTree + "/oracle":      {deps: set(module + "/engine/rules")},
		module + "/engine/rules":   {deps: set(module + "/engine/state")},
	})
	want := diracTree + "/search/hbot -> " + module + "/engine/state"
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %v, want [%s]", got, want)
	}
}

// rawTableReaders are the selectors that reach a set playing every lowered card,
// evidence or not.
var rawTableReaders = map[string]bool{
	"cardset.RawByName": true, "cardset.SynthByName": true, "cardset.Raw": true,
	"cardsettest.Synth": true,
}

// rawTableUsers may name them: lockstep earns evidence on raw tables, kgen builds
// them, and ten dirac tests read synthetic pools, nine for cards only they hold
// and one to measure them. Everything else plays what the support lists allow.
var rawTableUsers = []string{
	"dirac/oracle/", "dirac/kgen/", "dirac/cardset/",
	"dirac/panic_test.go", "dirac/poolbytes_test.go", "dirac/restractivation_test.go",
	"dirac/synthshapes_test.go", "dirac/xannounce_test.go", "dirac/grantcap_reader_test.go",
	"dirac/mana_grantedoffers_test.go", "dirac/graveact_restricted_test.go",
	"dirac/altadd_total_test.go", "dirac/mana_grantedrefused_test.go",
}

// rawTableViolation is the selector in src that file may not use, or "".
func rawTableViolation(file string, src []byte) string {
	for _, d := range rawTableUsers {
		if strings.HasPrefix(file, d) {
			return ""
		}
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	found := ""
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && found == "" {
			if x, ok := sel.X.(*ast.Ident); ok && rawTableReaders[x.Name+"."+sel.Sel.Name] {
				found = x.Name + "." + sel.Sel.Name
			}
		}
		return true
	})
	return found
}

func TestOnlyTheOracleReadsRawTables(t *testing.T) {
	walkPublicFiles(t, func(p string, blob []byte) {
		if !strings.HasSuffix(p, ".go") {
			return
		}
		if sel := rawTableViolation(p, blob); sel != "" {
			t.Errorf("%s uses %s: only the lockstep oracle and kgen read raw tables", p, sel)
		}
	})
}

func TestRawTableViolationDetection(t *testing.T) {
	src := []byte("package p\nfunc f() { cardset.RawByName(\"x\") }\n")
	for file, want := range map[string]string{
		"cmd/gorged/x.go":     "cardset.RawByName",
		"dirac/search/x.go":   "cardset.RawByName",
		"dirac/oracle/x.go":   "",
		"dirac/kgen/x.go":     "",
		"dirac/cardset/x.go":  "",
		"dirac/oraclish/x.go": "cardset.RawByName",
	} {
		if got := rawTableViolation(file, src); got != want {
			t.Errorf("%s: got %q, want %q", file, got, want)
		}
	}
}

// pointerFreeViolations walks typ and names each part that holds a reference,
// passing over the paths in skip.
func pointerFreeViolations(where string, typ types.Type, skip map[string]bool) []string {
	if skip[where] {
		return nil
	}
	switch u := typ.Underlying().(type) {
	case *types.Basic:
		if u.Kind() == types.String || u.Kind() == types.UnsafePointer {
			return []string{where + ": " + u.String()}
		}
		return nil
	case *types.Array:
		return pointerFreeViolations(where+"[]", u.Elem(), skip)
	case *types.Struct:
		var out []string
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			out = append(out, pointerFreeViolations(where+"."+f.Name(), f.Type(), skip)...)
		}
		return out
	default:
		return []string{fmt.Sprintf("%s: %s", where, u)}
	}
}

// stateReferences are the only fields of dirac.State that hold a reference: the
// views into its one variable buffer, and the pool that reuses States when forking.
var stateReferences = map[string]bool{"State.cold": true, "State.pool": true}

// stateViewViolations names each field of the views struct whose target holds a
// reference, or that is neither a slice nor a pointer: a view reaches pointer-free
// data and nothing else.
func stateViewViolations(views *types.Struct) []string {
	var out []string
	for i := 0; i < views.NumFields(); i++ {
		f := views.Field(i)
		where := "State.cold." + f.Name()
		switch u := f.Type().Underlying().(type) {
		case *types.Slice:
			out = append(out, pointerFreeViolations(where+"[]", u.Elem(), nil)...)
		case *types.Pointer:
			out = append(out, pointerFreeViolations("*"+where, u.Elem(), nil)...)
		default:
			out = append(out, pointerFreeViolations(where, f.Type(), nil)...)
		}
	}
	return out
}

// checkedPackage type-checks the non-test files of the package in dir.
func checkedPackage(t *testing.T, dir string) *types.Package {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	bp, err := build.ImportDir(abs, 0)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(abs, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), GoVersion: "go1.25"}
	checked, err := conf.Check(bp.ImportPath, fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	return checked
}

// TestStateIsPointerFree holds dirac.State's fixed core to scalars, arrays and
// structs, and its variable part to views of pointer-free data, so the collector
// has nothing to scan in a game's state however many States a search holds.
func TestStateIsPointerFree(t *testing.T) {
	requireDirac(t)
	checked := checkedPackage(t, filepath.Join("..", "..", "dirac"))
	obj := checked.Scope().Lookup("State")
	if obj == nil {
		t.Fatal("dirac has no type State")
	}
	bad := pointerFreeViolations("State", obj.Type(), stateReferences)
	state := obj.Type().Underlying().(*types.Struct)
	found := 0
	for i := 0; i < state.NumFields(); i++ {
		f := state.Field(i)
		if !stateReferences["State."+f.Name()] {
			continue
		}
		found++
		if views, ok := f.Type().Underlying().(*types.Struct); ok {
			bad = append(bad, stateViewViolations(views)...)
		}
	}
	if found != len(stateReferences) {
		t.Errorf("State has %d of the %d fields stateReferences names", found, len(stateReferences))
	}
	if len(bad) > 0 {
		t.Errorf("State holds references:\n\t%s", strings.Join(bad, "\n\t"))
	}
}

func TestPointerFreeViolationsDetection(t *testing.T) {
	const src = `package p
type clean struct { a [4]uint8; b struct{ c int16 } }
type ptr struct { a [2]struct{ b *int } }
type slice struct { a []int }
type str struct { a string }
type iface struct { a any }
type mp struct { a map[int]int }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := (&types.Config{}).Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"clean": false, "ptr": true, "slice": true, "str": true, "iface": true, "mp": true} {
		got := pointerFreeViolations(name, checked.Scope().Lookup(name).Type(), nil)
		if (len(got) > 0) != want {
			t.Errorf("%s: %v, want violation %v", name, got, want)
		}
	}
}

const poolInputsDir = "dirac/cardset/pools/"

// isGeneratedTableFile reports card data kgen writes under the gitignored corpus.
// Tables are mechanical translations of GPL-3.0 Forge scripts, so only a pool's
// card list and CardID index are committed.
func isGeneratedTableFile(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(p, "dirac/cards_") || strings.HasSuffix(base, ".ktab") ||
		strings.HasPrefix(p, poolInputsDir) && (strings.HasSuffix(base, "coverage.md") || base == "OPS.md")
}

func TestNoGeneratedTablesTracked(t *testing.T) {
	walkPublicFiles(t, func(p string, _ []byte) {
		if isGeneratedTableFile(p) {
			t.Errorf("%s is a generated card table: make dirac-tables writes these under the gitignored corpus", p)
		}
	})
}

func TestGeneratedTableDetection(t *testing.T) {
	for f, want := range map[string]bool{
		"dirac/cards_fdn/pool.go":                 true,
		"dirac/cardset/pools/fdn/fdn.ktab":        true,
		"somewhere/else.ktab":                     true,
		"dirac/cardset/pools/fdn/coverage.md":     true,
		"dirac/cardset/pools/fdn/fdn.coverage.md": true,
		"dirac/cardset/pools/fdn/OPS.md":          true,
		"dirac/cardset/pools/fdn/cards.txt":       false,
		"dirac/cardset/pools/fdn/ids.tsv":         false,
		"dirac/docs/coverage.md":                  false,
		"dirac/cardset/pooldata.go":               false,
	} {
		if got := isGeneratedTableFile(f); got != want {
			t.Errorf("%s: violation %v, want %v", f, got, want)
		}
	}
}
