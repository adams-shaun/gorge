package archtest

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdapterLinksNoRulesEngine holds a served dirac game solo: nothing the adapter
// links can reach gorge's rules engine.
func TestAdapterLinksNoRulesEngine(t *testing.T) {
	requireDirac(t)
	pkgs := packages(t)
	seen := map[string]bool{}
	var walk func(string)
	walk = func(p string) {
		if seen[p] {
			return
		}
		seen[p] = true
		if q, ok := pkgs[p]; ok {
			for imp := range q.imports {
				walk(imp)
			}
		}
	}
	root := module + "/dirac/adapter"
	if _, ok := pkgs[root]; !ok {
		t.Fatal("no dirac/adapter package")
	}
	walk(root)
	for _, banned := range []string{"/engine/rules", "/engine/replay", "/dirac/oracle"} {
		if seen[module+banned] {
			t.Errorf("dirac/adapter links %s", module+banned)
		}
	}
}

// TestAdapterBoardIsWrittenInOneFile: state mutation outside events.Apply is a replay
// bug everywhere but the adapter's board, a read-only image no game runs on. Only its
// own file writes it.
func TestAdapterBoardIsWrittenInOneFile(t *testing.T) {
	requireDirac(t)
	dir, err := filepath.Abs(filepath.Join("..", "..", "dirac", "adapter"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), GoVersion: "go1.25"}
	if _, err := conf.Check("adapter", fset, files, info); err != nil {
		t.Fatal(err)
	}
	isBoard := func(x ast.Expr) bool {
		tv, ok := info.Types[x]
		if !ok {
			return false
		}
		ty := tv.Type
		if p, ok := ty.(*types.Pointer); ok {
			ty = p.Elem()
		}
		n, ok := ty.(*types.Named)
		if !ok || n.Obj().Pkg() == nil || !strings.HasSuffix(n.Obj().Pkg().Path(), "/engine/state") {
			return false
		}
		switch n.Obj().Name() {
		case "Game", "Object", "Player":
			return true
		}
		return false
	}
	write := func(lhs ast.Expr) bool {
		for {
			switch x := lhs.(type) {
			case *ast.SelectorExpr:
				if isBoard(x.X) {
					return true
				}
				lhs = x.X
			case *ast.IndexExpr:
				lhs = x.X
			case *ast.ParenExpr:
				lhs = x.X
			default:
				return false
			}
		}
	}
	for _, f := range files {
		name := filepath.Base(fset.Position(f.Pos()).Filename)
		if name == "board.go" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if write(l) {
						t.Errorf("%s: writes the board outside board.go", fset.Position(l.Pos()))
					}
				}
			case *ast.IncDecStmt:
				if write(x.X) {
					t.Errorf("%s: writes the board outside board.go", fset.Position(x.Pos()))
				}
			case *ast.CallExpr:
				if s, ok := x.Fun.(*ast.SelectorExpr); ok && isBoard(s.X) {
					switch s.Sel.Name {
					case "AddObject", "SetZone", "AddCounter", "SyncFace":
						t.Errorf("%s: writes the board outside board.go", fset.Position(x.Pos()))
					}
				}
			}
			return true
		})
	}
}
