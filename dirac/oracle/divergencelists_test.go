package oracle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestLockstepConfigMatchesOracle: the kernel side the oracle plays is built from
// dirac.LockstepConfig on both surfaces, whichever factory makes it, and differs
// only in the fields lockstep leaves to its caller.
func TestLockstepConfigMatchesOracle(t *testing.T) {
	t.Parallel()
	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 1)[0]
	m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := KernelMatchup(c.Reg, vanilla.Table, [2]string{"vanilla0", "vanilla1"}, [2][]kop.CardID{vanilla.Decks[0], vanilla.Decks[1]}, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if dirac.LockstepConfig(dirac.SurfaceAutopay) == dirac.LockstepConfig(dirac.SurfaceFull) {
		t.Error("the two surfaces share one configuration")
	}
	for _, surface := range []dirac.Surface{dirac.SurfaceFull, dirac.SurfaceAutopay} {
		cfg := Config{GorgeAutopaySurface: surface == dirac.SurfaceAutopay, PassElision: true, RecordChance: true, ArrangeRestOrder: true}
		for name, mk := range map[string]func() (Side, error){
			"RealKernel": func() (Side, error) { return RealKernel(vanilla.Table, nil, cfg)(vm, 1) },
			"SetSide":    func() (Side, error) { return SetSide(rawSet("fdn"), tbl, cfg)(m, 1) },
		} {
			side, err := mk()
			if err != nil {
				t.Fatal(err)
			}
			got := side.(*realSide).kcfg
			if name != "RealKernel" { // vanilla.Table stays registered: parallel tests are playing it
				CloseSide(side)
			}
			if got.NoElide || !got.RecordChance || !got.ArrangeRestOrder {
				t.Errorf("%s surface %d: the caller's fields did not reach the kernel: %+v", name, surface, got)
			}
			got.NoElide, got.Mulligans, got.RecordChance, got.ArrangeRestOrder, got.AutoDivide = false, 0, false, false, false
			if want := dirac.LockstepConfig(surface); got != want {
				t.Errorf("%s surface %d: the kernel plays %+v, LockstepConfig is %+v", name, surface, got, want)
			}
		}
	}
}

// pinnedDivergenceLists is what TestNoKnownDivergenceLists finds. It only shrinks,
// and it ends empty: a supported card has no tolerated divergence.
var pinnedDivergenceLists = []string{
	"unmodelled_test.go: unmodelledKnown",
}

// TestNoKnownDivergenceLists walks the oracle's tests for a map named as a list of
// known divergences or stalls, and for a skip that names a gorge finding. Each hit
// is a difference a test tolerates. What it finds must equal the pinned list, so a
// new one fails, and so does a pin whose list is gone.
func TestNoKnownDivergenceLists(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		hits = append(hits, knownDivergenceHits(name, f)...)
	}
	slices.Sort(hits)
	hits = slices.Compact(hits)
	for _, h := range hits {
		t.Logf("tolerated: %s", h)
	}
	t.Logf("%d hits", len(hits))
	for _, h := range hits {
		if !slices.Contains(pinnedDivergenceLists, h) {
			t.Errorf("a new tolerated divergence: %s; fix it or refuse the card", h)
		}
	}
	for _, h := range pinnedDivergenceLists {
		if !slices.Contains(hits, h) {
			t.Errorf("%s is gone: remove it from pinnedDivergenceLists", h)
		}
	}
}

var knownName = regexp.MustCompile(`(?i)known`)

// knownDivergenceHits lists f's maps with a known-divergence name (an "unknown"
// is something else) and its skips that name a gorge finding.
func knownDivergenceHits(file string, f *ast.File) []string {
	var hits []string
	isList := func(name string) bool {
		return knownName.MatchString(name) && !strings.Contains(strings.ToLower(name), "unknown")
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			for i, id := range x.Names {
				if !isList(id.Name) {
					continue
				}
				_, typed := x.Type.(*ast.MapType)
				if typed || i < len(x.Values) && isMapValue(x.Values[i]) {
					hits = append(hits, file+": "+id.Name)
				}
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				if id, ok := l.(*ast.Ident); ok && isList(id.Name) && i < len(x.Rhs) && isMapValue(x.Rhs[i]) {
					hits = append(hits, file+": "+id.Name)
				}
			}
		case *ast.Field:
			if _, ok := x.Type.(*ast.MapType); ok {
				for _, id := range x.Names {
					if isList(id.Name) {
						hits = append(hits, file+": "+id.Name)
					}
				}
			}
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "Skip") {
				return true
			}
			for _, a := range x.Args {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(strings.ToLower(s), "gorge finding") {
						hits = append(hits, file+": skip")
					}
				}
			}
		}
		return true
	})
	return hits
}

// isMapValue reports whether e is a map literal or a make of a map.
func isMapValue(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.CompositeLit:
		_, ok := v.Type.(*ast.MapType)
		return ok
	case *ast.CallExpr:
		if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "make" && len(v.Args) > 0 {
			_, ok := v.Args[0].(*ast.MapType)
			return ok
		}
	}
	return false
}
