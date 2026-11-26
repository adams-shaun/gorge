package codeshape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
)

// ShapeDirs are the repo-relative trees the per-function growth rule holds
// (cmd/funcgrowth, backed by CheckFuncGrowth): the rules engine and the whole
// of dirac, kgen included. cmd/funcgrowth diffs them against the base commit;
// the walk skips test files and generated files (the standard "Code generated
// ... DO NOT EDIT." header).
var ShapeDirs = []string{"engine/rules", "engine/effects", "dirac"}

// ComplexFuncThreshold is the cyclomatic complexity above which a function
// counts as complex.
const ComplexFuncThreshold = 50

// FuncShape is one function declaration's length and cyclomatic complexity.
type FuncShape struct {
	Name       string // receiver-qualified, as QualName
	File       string // repo-relative, slash-separated
	Line       int    // line of the func keyword
	Lines      int    // end line - start line + 1
	Complexity int    // Cyclomatic
}

// Key is the function's ceiling key, "<dir> <Name>" (as Func.CeilingKey):
// moving a function between files of one package keeps its key.
func (f FuncShape) Key() string { return path.Dir(f.File) + " " + f.Name }

// Cyclomatic is a function's cyclomatic complexity as gocyclo counts it:
// 1, plus one per if, for, range, non-default case or select clause, and
// && or ||. Function literals inside count toward the enclosing function.
func Cyclomatic(fn *ast.FuncDecl) int {
	c := 1
	if fn.Body == nil {
		return c
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if x.List != nil {
				c++
			}
		case *ast.CommClause:
			if x.Comm != nil {
				c++
			}
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

// shapesByKey folds the measured functions by key, keeping the larger of two
// that share one (a name declared in two build-tagged files of one package).
func shapesByKey(shapes []FuncShape) (map[string]FuncShape, []string) {
	by := map[string]FuncShape{}
	var keys []string
	for _, s := range shapes {
		k := s.Key()
		old, seen := by[k]
		if !seen {
			keys = append(keys, k)
		}
		if !seen || s.Lines > old.Lines || s.Complexity > old.Complexity {
			s.Lines, s.Complexity = max(s.Lines, old.Lines), max(s.Complexity, old.Complexity)
			by[k] = s
		}
	}
	sort.Strings(keys)
	return by, keys
}

// MeasureFuncShapesIn measures every non-test function declared in files —
// the map's keys are repo-relative slash-separated paths, its values the
// files' absolute paths — with no threshold filter, sorted by key then file
// and line. cmd/funcgrowth measures the changed files of a diff through it,
// at the base (extracted from git) and at the head.
func MeasureFuncShapesIn(files map[string]string) ([]FuncShape, error) {
	rels := make([]string, 0, len(files))
	for rel := range files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	var out []FuncShape
	for _, rel := range rels {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, files[rel], nil,
			parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("codeshape: parse %s: %w", rel, err)
		}
		if ast.IsGenerated(f) {
			continue
		}
		for _, decl := range f.Decls {
			d, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			start := fset.Position(d.Pos()).Line
			out = append(out, FuncShape{Name: QualName(d), File: rel, Line: start,
				Lines: fset.Position(d.End()).Line - start + 1, Complexity: Cyclomatic(d)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Key() != b.Key() {
			return a.Key() < b.Key()
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return out, nil
}
