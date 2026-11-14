package codeshape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
)

// ShapeDirs are the repo-relative trees the per-function ratchets hold
// (TestLongFunctionsOnlyShrink, TestComplexFunctionsOnlyShrink): the rules
// engine and the whole of dirac, kgen included. Each is walked as goFiles
// walks ScannedDirs; test files and generated files (the standard
// "Code generated ... DO NOT EDIT." header) are skipped.
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

// MeasureFuncShapes returns every non-test, non-generated function under
// dirs (relative to root) that is over LongFuncLines lines or over
// ComplexFuncThreshold, sorted by key then file and line. It parses syntax
// only, so it is fast and needs no build.
func MeasureFuncShapes(root string, dirs []string) ([]FuncShape, error) {
	var out []FuncShape
	for _, dir := range dirs {
		files, err := goFiles(root, dir)
		if err != nil {
			return nil, err
		}
		for _, rel := range files {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil,
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
				s := FuncShape{Name: QualName(d), File: rel, Line: start,
					Lines: fset.Position(d.End()).Line - start + 1, Complexity: Cyclomatic(d)}
				if s.Lines > LongFuncLines || s.Complexity > ComplexFuncThreshold {
					out = append(out, s)
				}
			}
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
