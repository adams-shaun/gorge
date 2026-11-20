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

// The directives a kernel handler carries in its doc comment. Each names one
// code, and gen writes the generated switch that calls the handler, so a new
// code touches only its own feature file.
//
//	//kmanifest:ext ExtFoo
//	func (s *State) extFoo(k *prep, in *kop.Instr) (ok, done bool) { ... }
//
// ext: a sub-op with no case in the hand switch (execExt, dirac/extop.go);
// gen writes execExtGen, which that switch's default arm calls.
//
//	//kmanifest:op OpFoo
//	func (s *State) foo(k *prep, in *kop.Instr) (ok, done bool) { ... }
//
// op: a hot-band kop.Op no hand switch (an exec* method) has a case for;
// gen writes execOpGen, which the cold op chain ends in.
//
//	//kmanifest:askopts raFoo
//	func (s *State) fooOptions(k *prep, d *Decision) { ... }
//	//kmanifest:askapply raFoo
//	func (s *State) fooAnswer(k *prep, a Action) { ... }
//
// askopts and askapply: a resolution ask kind (resCtx.ask) the hand chains
// (askOptionsCold1, applyAskCold1) have no case for, with the builder of its
// options and the answer's handler; gen writes askOptionsGen and applyAskGen,
// which those chains end in. An ask has both directives or neither.
//
//	//kmanifest:fword FwFoo
//	func (s *State) fwFoo(c *fctx, l *kop.FLit) bool { ... }
//
// fword: a filter word (kop.FilterWord, a literal of a PredProg program, one
// file per word, dirac/filter_<name>.go); gen writes fwTable, the package data
// filterProg indexes by word.
const (
	ExtDirective      = "//kmanifest:ext"
	OpDirective       = "//kmanifest:op"
	AskOptsDirective  = "//kmanifest:askopts"
	AskApplyDirective = "//kmanifest:askapply"
	FWordDirective    = "//kmanifest:fword"
)

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

// OpHandler is one //kmanifest:op handler.
type OpHandler struct {
	Op     string // the kop.Op constant, e.g. OpFoo
	Method string // the State method that runs it
	File   string // its file, a slash path from the module root
}

// AskHandler is one ask kind's two handlers.
type AskHandler struct {
	Ask         string // the ask constant, e.g. raFoo
	Options     string // the State method that builds the options
	Apply       string // the State method that applies the answer
	OptionsFile string // the files they are in, slash paths from the module root
	ApplyFile   string
}

// WordHandler is one //kmanifest:fword handler.
type WordHandler struct {
	Word   string // the kop.FilterWord constant, e.g. FwFoo
	Method string // the State method that runs it
	File   string // its file, a slash path from the module root
}

// Handlers are the kernel's annotated handlers, each list sorted by code.
type Handlers struct {
	Ext  []ExtHandler
	Op   []OpHandler
	Ask  []AskHandler
	Word []WordHandler
}

// dirKind is what one directive wants of its handler.
type dirKind struct {
	directive string
	prefix    string // the code constant's name starts with it
	what      string // for the error text
	params    string // the handler's parameter types, comma-joined
	results   string // its result types, comma-joined ("" for none)
	want      string // the signature, for the error text
}

var (
	extKind = dirKind{ExtDirective, "Ext", "ExtOp", "*prep,*kop.Instr", "bool,bool", "is not a func (*State) (*prep, *kop.Instr) (bool, bool) method"}
	opKind  = dirKind{OpDirective, "Op", "Op", "*prep,*kop.Instr", "bool,bool", "is not a func (*State) (*prep, *kop.Instr) (bool, bool) method"}
	optKind = dirKind{AskOptsDirective, "ra", "ask", "*prep,*Decision", "", "is not a func (*State) (*prep, *Decision) method"}
	appKind = dirKind{AskApplyDirective, "ra", "ask", "*prep,Action", "", "is not a func (*State) (*prep, Action) method"}
	fwKind  = dirKind{FWordDirective, "Fw", "FilterWord", "*fctx,*kop.FLit", "bool", "is not a func (*State) (*fctx, *kop.FLit) bool method"}
)

var dirKinds = []dirKind{extKind, opKind, optKind, appKind, fwKind}

// found is one handler a scan met: its directive's kind, the code, the
// method and the file.
type found struct {
	kind         dirKind
	code, method string
	file         string
}

// ScanExt is ScanKernel's //kmanifest:ext handlers, sorted by op.
func ScanExt(fsys fs.FS) ([]ExtHandler, error) {
	h, err := ScanKernel(fsys)
	return h.Ext, err
}

// ScanKernel finds the //kmanifest handlers in package dirac's non-test,
// non-generated source of fsys (a tree rooted at the module). A handler must
// be a State method with its directive's signature, a code may have one
// handler per directive, an ask has both of its directives, and a code the
// hand switches already dispatch may have none. A tree with no dirac
// directory has no handlers.
func ScanKernel(fsys fs.FS) (Handlers, error) {
	des, err := fs.ReadDir(fsys, KernelDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Handlers{}, nil
		}
		return Handlers{}, err
	}
	var all []found
	var errs []string
	src := map[string][]byte{}
	fset := token.NewFileSet()
	for _, de := range des {
		n := de.Name()
		p := path.Join(KernelDir, n)
		if de.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || p == DiracGen {
			continue
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return Handlers{}, err
		}
		src[p] = b
		if !bytes.Contains(b, []byte("//kmanifest:")) {
			continue
		}
		fh, es, err := scanFile(fset, p, b)
		if err != nil {
			return Handlers{}, err
		}
		all, errs = append(all, fh...), append(errs, es...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].code != all[j].code {
			return all[i].code < all[j].code
		}
		return all[i].kind.directive < all[j].kind.directive
	})
	var h Handlers
	for i, f := range all {
		if i > 0 && all[i-1].code == f.code && all[i-1].kind == f.kind {
			errs = append(errs, fmt.Sprintf("%s: %s has two handlers: %s (%s) and %s (%s)", KernelDir, f.code, all[i-1].method, all[i-1].file, f.method, f.file))
		}
		switch f.kind {
		case extKind:
			h.Ext = append(h.Ext, ExtHandler{Op: f.code, Method: f.method, File: f.file})
		case opKind:
			h.Op = append(h.Op, OpHandler{Op: f.code, Method: f.method, File: f.file})
		case fwKind:
			h.Word = append(h.Word, WordHandler{Word: f.code, Method: f.method, File: f.file})
		}
	}
	asks, askErrs := pairAsks(all)
	h.Ask = asks
	errs = append(errs, askErrs...)
	errs = append(errs, handCaseErrs(fsys, fset, src, h)...)
	if len(errs) > 0 {
		sort.Strings(errs)
		return Handlers{}, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return h, nil
}

// scanFile is the handlers of one file's source, and the misuses of any
// directive in it.
func scanFile(fset *token.FileSet, p string, b []byte) ([]found, []string, error) {
	f, err := parser.ParseFile(fset, p, b, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	var out []found
	var errs []string
	marked := map[*ast.Comment]bool{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Doc == nil {
			continue
		}
		for _, c := range fd.Doc.List {
			k, code, ok := directiveOf(c.Text)
			if !ok {
				continue
			}
			marked[c] = true
			at := fset.Position(c.Slash)
			if !token.IsIdentifier(code) || !strings.HasPrefix(code, k.prefix) || len(code) == len(k.prefix) {
				errs = append(errs, fmt.Sprintf("%s:%d: %s wants one %s constant name, got %q", at.Filename, at.Line, k.directive, k.what, code))
				continue
			}
			if why := k.signature(fd); why != "" {
				errs = append(errs, fmt.Sprintf("%s:%d: %s %s: %s %s", at.Filename, at.Line, k.directive, code, fd.Name.Name, why))
				continue
			}
			out = append(out, found{kind: k, code: code, method: fd.Name.Name, file: p})
		}
	}
	// A directive anywhere else (a type, a statement, a stray line) would
	// silently dispatch nothing.
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if k, _, ok := directiveOf(c.Text); ok && !marked[c] {
				at := fset.Position(c.Slash)
				errs = append(errs, fmt.Sprintf("%s:%d: %s is not in a function's doc comment", at.Filename, at.Line, k.directive))
			}
		}
	}
	return out, errs, nil
}

// directiveOf is the kind and the code a //kmanifest comment names.
func directiveOf(text string) (dirKind, string, bool) {
	for _, k := range dirKinds {
		rest, ok := strings.CutPrefix(text, k.directive)
		if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			continue
		}
		return k, strings.TrimSpace(rest), true
	}
	return dirKind{}, "", false
}

// pairAsks joins each ask's two handlers; an ask with one is an error naming
// its file.
func pairAsks(all []found) ([]AskHandler, []string) {
	by := map[string]*AskHandler{}
	var order []string
	var errs []string
	for _, f := range all {
		if f.kind != optKind && f.kind != appKind {
			continue
		}
		a := by[f.code]
		if a == nil {
			a = &AskHandler{Ask: f.code}
			by[f.code] = a
			order = append(order, f.code)
		}
		if f.kind == optKind {
			a.Options, a.OptionsFile = f.method, f.file
		} else {
			a.Apply, a.ApplyFile = f.method, f.file
		}
	}
	var out []AskHandler
	for _, c := range order {
		a := by[c]
		switch {
		case a.Options == "":
			errs = append(errs, fmt.Sprintf("%s: %s has %s %s but no %s", a.ApplyFile, c, AskApplyDirective, a.Apply, AskOptsDirective))
		case a.Apply == "":
			errs = append(errs, fmt.Sprintf("%s: %s has %s %s but no %s", a.OptionsFile, c, AskOptsDirective, a.Options, AskApplyDirective))
		default:
			out = append(out, *a)
		}
	}
	return out, errs
}

// signature is why fd is not a State method of k's shape, or "".
func (k dirKind) signature(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || exprString(fd.Recv.List[0].Type) != "*State" {
		return k.want
	}
	if strings.Join(fieldTypes(fd.Type.Params), ",") != k.params || strings.Join(fieldTypes(fd.Type.Results), ",") != k.results {
		return k.want
	}
	return ""
}

func fieldTypes(fl *ast.FieldList) []string {
	var out []string
	if fl == nil {
		return nil
	}
	for _, f := range fl.List {
		for range max(len(f.Names), 1) {
			out = append(out, exprString(f.Type))
		}
	}
	return out
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

// handCaseErrs is a handler whose code a hand switch already has a case for:
// the hand arm runs first, so the handler would never run.
func handCaseErrs(fsys fs.FS, fset *token.FileSet, src map[string][]byte, h Handlers) []string {
	var errs []string
	hand, err := extHandCases(fsys, fset)
	if err != nil {
		return []string{err.Error()}
	}
	for _, x := range h.Ext {
		if hand[x.Op] {
			errs = append(errs, fmt.Sprintf("%s: %s already has a case in %s (%s), so its %s handler %s would never run", x.File, x.Op, extHandFunc, extHandFile, ExtDirective, x.Method))
		}
	}
	if len(h.Op) == 0 && len(h.Ask) == 0 {
		return errs
	}
	ops, asks := map[string]OpHandler{}, map[string]AskHandler{}
	for _, o := range h.Op {
		ops[o.Op] = o
	}
	for _, a := range h.Ask {
		asks[a.Ask] = a
	}
	paths := make([]string, 0, len(src))
	for p := range src {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		b := src[p]
		if !mentionsAny(b, ops, asks) {
			continue
		}
		f, err := parser.ParseFile(fset, p, b, parser.SkipObjectResolution)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Recv == nil || len(fd.Recv.List) != 1 || exprString(fd.Recv.List[0].Type) != "*State" {
				continue
			}
			name := fd.Name.Name
			isOp := strings.HasPrefix(name, "exec")
			isAsk := false
			for _, pre := range []string{"askOptions", "applyAsk", "resolveOptions", "applyResolve"} {
				isAsk = isAsk || strings.HasPrefix(name, pre)
			}
			if !isOp && !isAsk {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				cc, ok := n.(*ast.CaseClause)
				if !ok {
					return true
				}
				for _, e := range cc.List {
					switch e := e.(type) {
					case *ast.SelectorExpr:
						if o, ok := ops[e.Sel.Name]; ok && isOp && exprString(e.X) == "kop" {
							errs = append(errs, fmt.Sprintf("%s: %s already has a case in %s (%s), so its %s handler %s would never run", o.File, o.Op, name, p, OpDirective, o.Method))
						}
					case *ast.Ident:
						if a, ok := asks[e.Name]; ok && isAsk {
							errs = append(errs, fmt.Sprintf("%s: %s already has a case in %s (%s), so its %s handlers %s and %s would never run", a.OptionsFile, a.Ask, name, p, AskOptsDirective, a.Options, a.Apply))
						}
					}
				}
				return true
			})
		}
	}
	return errs
}

func mentionsAny(b []byte, ops map[string]OpHandler, asks map[string]AskHandler) bool {
	for o := range ops {
		if bytes.Contains(b, []byte(o)) {
			return true
		}
	}
	for a := range asks {
		if bytes.Contains(b, []byte(a)) {
			return true
		}
	}
	return false
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

// genKernelDispatch is the generated switches the hand chains end in:
// execExtGen, execOpGen, askOptionsGen and applyAskGen.
func genKernelDispatch(h Handlers) string {
	var b strings.Builder
	b.WriteString("\n// execExtGen runs an OpExt sub-op whose handler is marked " + ExtDirective + " in its\n" +
		"// feature file: the default arm of the hand switch (execExt, extop.go).\n" +
		"//\n//go:noinline\n" +
		"func (s *State) execExtGen(k *prep, in *kop.Instr) (ok, done bool) {\n")
	if len(h.Ext) > 0 {
		b.WriteString("\tswitch kop.ExtOp(in.A.N) {\n")
		for _, x := range h.Ext {
			fmt.Fprintf(&b, "\tcase kop.%s:\n\t\treturn s.%s(k, in) // %s\n", x.Op, x.Method, path.Base(x.File))
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("\tpanic(\"kernel: ext op not implemented: \" + kop.ExtOp(in.A.N).String())\n}\n")

	b.WriteString("\n// execOpGen runs a hot-band op whose handler is marked " + OpDirective + " in its\n" +
		"// feature file: the cold op chain's last call (execGen); done=false: not one of them.\n" +
		"//\n//go:noinline\n" +
		"func (s *State) execOpGen(k *prep, in *kop.Instr) (ok, done bool) {\n\tswitch in.Op {\n")
	for _, x := range h.Op {
		fmt.Fprintf(&b, "\tcase kop.%s:\n\t\treturn s.%s(k, in) // %s\n", x.Op, x.Method, path.Base(x.File))
	}
	b.WriteString("\t}\n\treturn true, false\n}\n")

	// The switch and the final return are there with no case, so the first
	// handler of a kind only adds lines.
	askFn := func(doc, name, sig string, call func(AskHandler) (string, string)) {
		b.WriteString("\n" + doc + "//\n//go:noinline\nfunc (s *State) " + name + "(" + sig + ") bool {\n\tswitch s.res.ask {\n")
		for _, a := range h.Ask {
			m, f := call(a)
			fmt.Fprintf(&b, "\tcase %s:\n\t\ts.%s // %s\n\t\treturn true\n", a.Ask, m, path.Base(f))
		}
		b.WriteString("\t}\n\treturn false\n}\n")
	}
	askFn("// askOptionsGen builds the options of a resolution ask whose handlers are marked\n"+
		"// "+AskOptsDirective+" in their feature file (askOptionsCold1's last call); false: not one of them.\n",
		"askOptionsGen", "k *prep, d *Decision",
		func(a AskHandler) (string, string) { return a.Options + "(k, d)", a.OptionsFile })
	askFn("// applyAskGen applies an answer to a resolution ask whose handlers are marked\n"+
		"// "+AskApplyDirective+" in their feature file (applyAskCold1's last call); false: not one of them.\n",
		"applyAskGen", "k *prep, a Action",
		func(a AskHandler) (string, string) { return a.Apply + "(k, a)", a.ApplyFile })
	return b.String()
}

// genFilterWords is fwRun and fwHandled, the dispatch of the //kmanifest:fword
// handlers. fwRun is a switch, not a table of method values: a call through a
// func value makes its *fctx argument escape, an allocation per literal, and a
// direct call does not. A word with no handler panics in fwRun, and
// fwHandled is false for it, which table prep (checkFilterProgs) refuses.
func genFilterWords(h Handlers) string {
	var b strings.Builder
	b.WriteString("\n// fwRun runs a filter program literal's word, whose handler is marked " + FWordDirective + " in its\n" +
		"// feature file (filterProg's dispatch).\n" +
		"func (s *State) fwRun(c *fctx, l *kop.FLit) bool {\n\tswitch l.W {\n")
	for _, w := range h.Word {
		fmt.Fprintf(&b, "\tcase kop.%s:\n\t\treturn s.%s(c, l) // %s\n", w.Word, w.Method, path.Base(w.File))
	}
	b.WriteString("\t}\n\tpanic(\"kernel: filter word not implemented\")\n}\n")
	b.WriteString("\n// fwHandled reports whether word w has a handler.\nfunc fwHandled(w kop.FilterWord) bool {\n\tswitch w {\n")
	if len(h.Word) > 0 {
		b.WriteString("\tcase ")
		for i, w := range h.Word {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("kop." + w.Word)
		}
		b.WriteString(":\n\t\treturn true\n")
	}
	b.WriteString("\t}\n\treturn false\n}\n")
	return b.String()
}
