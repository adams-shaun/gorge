package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// ExtDirective marks a kernel handler of an extension sub-op that has no case
// in the hand switch (execExt, dirac/extop.go):
//
//	//kmanifest:ext ExtFoo
//	func (s *State) extFoo(k *prep, in *kop.Instr) (ok, done bool) { ... }
//
// gen writes execExtGen, the switch the hand switch's default arm calls, with
// one case per annotated handler, so activating a sub-op touches only its own
// feature file.
const ExtDirective = "//kmanifest:ext"

// KernelDir is package dirac's directory, a slash path from the module root.
const KernelDir = "dirac"

// extHandSwitch is the file and function holding the hand switch.
const (
	extHandFile = KernelDir + "/extop.go"
	extHandFunc = "execExt"
)

// ExtHandler is one annotated handler.
type ExtHandler struct {
	Op     string // the kop.ExtOp constant, e.g. ExtFoo
	Method string // the State method that runs it
	File   string // its file, a slash path from the module root
}

// ScanExt finds the //kmanifest:ext handlers in package dirac's non-test
// source of fsys (a tree rooted at the module), sorted by op. A handler must
// be a State method with execExt's signature, an op may have one handler, and
// an op the hand switch already dispatches may have none. A tree with no
// dirac directory has no handlers.
func ScanExt(fsys fs.FS) ([]ExtHandler, error) {
	des, err := fs.ReadDir(fsys, KernelDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []ExtHandler
	var errs []string
	fset := token.NewFileSet()
	for _, de := range des {
		n := de.Name()
		if de.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || path.Join(KernelDir, n) == DiracGen {
			continue
		}
		p := path.Join(KernelDir, n)
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		if !bytes.Contains(b, []byte(ExtDirective)) {
			continue
		}
		f, err := parser.ParseFile(fset, p, b, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		marked := map[*ast.Comment]bool{}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Doc == nil {
				continue
			}
			for _, c := range fd.Doc.List {
				op, ok := extDirective(c.Text)
				if !ok {
					continue
				}
				marked[c] = true
				at := fset.Position(c.Slash)
				if !token.IsIdentifier(op) || !strings.HasPrefix(op, "Ext") || len(op) == len("Ext") {
					errs = append(errs, fmt.Sprintf("%s:%d: %s wants one ExtOp constant name, got %q", at.Filename, at.Line, ExtDirective, op))
					continue
				}
				if why := extSignature(fd); why != "" {
					errs = append(errs, fmt.Sprintf("%s:%d: %s %s: %s %s", at.Filename, at.Line, ExtDirective, op, fd.Name.Name, why))
					continue
				}
				out = append(out, ExtHandler{Op: op, Method: fd.Name.Name, File: p})
			}
		}
		// A directive anywhere else (a type, a statement, a stray line) would
		// silently dispatch nothing.
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if _, ok := extDirective(c.Text); ok && !marked[c] {
					at := fset.Position(c.Slash)
					errs = append(errs, fmt.Sprintf("%s:%d: %s is not in a function's doc comment", at.Filename, at.Line, ExtDirective))
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Op < out[j].Op })
	for i := 1; i < len(out); i++ {
		if out[i].Op == out[i-1].Op {
			errs = append(errs, fmt.Sprintf("%s: %s has two handlers: %s (%s) and %s (%s)", KernelDir, out[i].Op, out[i-1].Method, out[i-1].File, out[i].Method, out[i].File))
		}
	}
	hand, err := extHandCases(fsys, fset)
	if err != nil {
		return nil, err
	}
	for _, h := range out {
		if hand[h.Op] {
			errs = append(errs, fmt.Sprintf("%s: %s already has a case in %s (%s), so its %s handler %s would never run", h.File, h.Op, extHandFunc, extHandFile, ExtDirective, h.Method))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return out, nil
}

// extDirective is the op a //kmanifest:ext comment names.
func extDirective(text string) (string, bool) {
	rest, ok := strings.CutPrefix(text, ExtDirective)
	if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// extSignature is why fd is not func (s *State) m(k *prep, in *kop.Instr)
// (ok, done bool), or "".
func extSignature(fd *ast.FuncDecl) string {
	const want = "is not a func (*State) (*prep, *kop.Instr) (bool, bool) method"
	if fd.Recv == nil || len(fd.Recv.List) != 1 || exprString(fd.Recv.List[0].Type) != "*State" {
		return want
	}
	var params, results []string
	for _, f := range fd.Type.Params.List {
		for range max(len(f.Names), 1) {
			params = append(params, exprString(f.Type))
		}
	}
	if fd.Type.Results != nil {
		for _, f := range fd.Type.Results.List {
			for range max(len(f.Names), 1) {
				results = append(results, exprString(f.Type))
			}
		}
	}
	if strings.Join(params, ",") != "*prep,*kop.Instr" || strings.Join(results, ",") != "bool,bool" {
		return want
	}
	return ""
}

func exprString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	}
	return fmt.Sprintf("%T", e)
}

// extHandCases is the ops the hand switch has a case for (none when the tree
// has no extop.go).
func extHandCases(fsys fs.FS, fset *token.FileSet) (map[string]bool, error) {
	b, err := fs.ReadFile(fsys, extHandFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	f, err := parser.ParseFile(fset, extHandFile, b, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	cases := map[string]bool{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != extHandFunc || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List {
				if s, ok := e.(*ast.SelectorExpr); ok && exprString(s.X) == "kop" {
					cases[s.Sel.Name] = true
				}
			}
			return true
		})
	}
	return cases, nil
}

// genExtDispatch is execExtGen: the hand switch's default arm, one case per
// handler, and the not-implemented panic the hand switch used to end with.
func genExtDispatch(hs []ExtHandler) string {
	var b strings.Builder
	b.WriteString("\n// execExtGen runs an OpExt sub-op whose handler is marked " + ExtDirective + " in its\n" +
		"// feature file: the default arm of the hand switch (execExt, extop.go).\n" +
		"//\n//go:noinline\n" +
		"func (s *State) execExtGen(k *prep, in *kop.Instr) (ok, done bool) {\n")
	if len(hs) > 0 {
		b.WriteString("\tswitch kop.ExtOp(in.A.N) {\n")
		for _, h := range hs {
			fmt.Fprintf(&b, "\tcase kop.%s:\n\t\treturn s.%s(k, in) // %s\n", h.Op, h.Method, path.Base(h.File))
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\tpanic(\"kernel: ext op not implemented: \" + kop.ExtOp(in.A.N).String())\n}\n")
	return b.String()
}
