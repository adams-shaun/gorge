package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// AdoptCase is how Adopt resolved one hand constant.
type AdoptCase uint8

const (
	// AdoptSameName: an entry of the same name exists; its generated
	// constant replaces the hand declaration.
	AdoptSameName AdoptCase = iota + 1
	// AdoptRenamed: an entry for the same vocabulary item exists under
	// another name; the unit's uses are rewritten to it (gofmt -r).
	AdoptRenamed
	// AdoptNext: no entry: the declaration becomes a value: next entry.
	AdoptNext
)

func (c AdoptCase) String() string {
	return [...]string{"", "same name", "renamed", "value: next"}[c]
}

// Adoption is one hand constant Adopt moved onto the manifest.
type Adoption struct {
	Name  string // the hand constant
	Entry string // the entry's path
	Case  AdoptCase
	// Active reports whether the entry is active afterwards: the kernel
	// (package dirac) references its constant.
	Active bool
}

// kopDir is package kop's directory, a slash path from the module root.
const kopDir = "dirac/kop"

// handConst is one hand-written kop constant of a manifest family.
type handConst struct {
	name, family string
	file         string // slash path from the module root
	doc          string
	value        int64 // from the unit's ordinals.tsv, -1 when it has no row
	forge        []string
}

// Adopt moves a rebased unit's hand-written kop constants of manifest
// families onto the manifest (the module at root; base is the tree it was
// rebased onto). A constant is the unit's when package kop declares it by
// hand and base has no ordinals row for it, or base's row is a manifest
// entry's. For each, the hand declaration and its name registrations
// (extOpNames[X] = ..., opNames[X] = ..., CtrXNames elements the unit
// appended, kgen's "kop.X", kop.X pairs) are deleted, and:
//
//   - an entry of the same name: its generated constant takes over;
//   - an entry of the same family whose forge token is the constant's
//     (its CtrXNames or extOpNames string, or its name past the family
//     prefix): the uses in dirac are rewritten to the entry's name with
//     gofmt -r;
//   - no entry: the declaration becomes a value: next entry carrying its
//     forge token and doc comment, for Assign to number.
//
// An entry becomes active when the kernel (package dirac) references its
// constant. Adopt then regenerates the generated files (WriteGen). A
// constant it cannot resolve (two entries match, a declaration other
// constants repeat) is an error naming it, and nothing is written.
func Adopt(root string, base *Tree) ([]Adoption, error) {
	t, err := Read(os.DirFS(root))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	src := map[string][]byte{}       // slash path -> current bytes
	cut := map[string][][2]int{}     // slash path -> byte ranges to delete
	parsed := map[string]*ast.File{} // kop and kgen files
	readGo := func(dir string) ([]string, error) {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		if err != nil {
			return nil, err
		}
		var out []string
		for _, p := range files {
			slash := path.Join(dir, filepath.Base(p))
			if strings.HasSuffix(p, "_test.go") || isGenFile(slash) {
				continue
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			f, err := parser.ParseFile(fset, slash, b, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			src[slash], parsed[slash] = b, f
			out = append(out, slash)
		}
		return out, nil
	}
	kopFiles, err := readGo(kopDir)
	if err != nil {
		return nil, err
	}
	kgenFiles, err := readGo("dirac/kgen")
	if err != nil {
		return nil, err
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }

	// The unit's hand constants.
	var hand []*handConst
	var errs []error
	for _, p := range kopFiles {
		for _, d := range parsed[p].Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			var typ ast.Expr
			for i, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				if vs.Values != nil {
					typ = vs.Type
				}
				id, ok := typ.(*ast.Ident)
				if !ok {
					continue
				}
				f := FamilyNamed(id.Name)
				if f == nil || f.Pkg != "kop" {
					continue
				}
				for _, n := range vs.Names {
					if r := base.byName[n.Name]; r != nil && base.entryNamedIn("kop", n.Name) == nil {
						continue // a constant the base has by hand
					}
					if len(vs.Names) > 1 {
						errs = append(errs, fmt.Errorf("%s: %s: adopt takes one constant per spec", p, n.Name))
						continue
					}
					if vs.Values != nil && i+1 < len(g.Specs) && g.Specs[i+1].(*ast.ValueSpec).Values == nil {
						next := g.Specs[i+1].(*ast.ValueSpec).Names[0].Name
						errs = append(errs, fmt.Errorf("%s: cannot remove %s: %s repeats its expression; give %s its own value", p, n.Name, next, next))
						continue
					}
					h := &handConst{name: n.Name, family: f.Name, file: p, value: -1}
					if r := t.byName[n.Name]; r != nil && r.Type == f.Name {
						if v, err := parseOrdinal(r.Value); err == nil {
							h.value = v
						}
					}
					doc := vs.Doc
					if doc == nil && len(g.Specs) == 1 {
						doc = g.Doc
					}
					if doc != nil {
						h.doc = strings.Join(strings.Fields(doc.Text()), " ")
					}
					hand = append(hand, h)
					cut[p] = append(cut[p], specRange(src[p], g, i, off))
				}
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if len(hand) == 0 {
		return nil, nil
	}
	byName := map[string]*handConst{}
	for _, h := range hand {
		byName[h.name] = h
	}

	// Registrations: X's name-table assignments in kop, the CtrXNames
	// elements the unit appended, and kgen's "kop.X", kop.X pairs.
	baseLen, err := baseCtrXNamesLen(root, base)
	if err != nil {
		return nil, err
	}
	charge, hasCharge := int64(-1), false
	if r := t.byName["CtrCharge"]; r != nil {
		if v, err := parseOrdinal(r.Value); err == nil {
			charge, hasCharge = v, true
		}
	}
	for _, p := range kopFiles {
		for _, d := range parsed[p].Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Name.Name != "init" || d.Recv != nil || d.Body == nil {
					continue
				}
				kept := 0
				var drop [][2]int
				for _, st := range d.Body.List {
					if h := registers(st, byName); h != nil {
						if s := regString(st); s != "" {
							h.forge = append(h.forge, s)
						}
						drop = append(drop, [2]int{lineStart(src[p], off(st.Pos())), lineEnd(src[p], off(st.End()))})
						continue
					}
					kept++
				}
				if kept == 0 && len(drop) > 0 {
					drop = [][2]int{declRange(src[p], d.Doc, d.Pos(), d.End(), off)}
				}
				cut[p] = append(cut[p], drop...)
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "CtrXNames" || len(vs.Values) != 1 {
						continue
					}
					lit, ok := vs.Values[0].(*ast.CompositeLit)
					if !ok || !hasCharge {
						continue
					}
					for _, h := range hand {
						if h.family != "CounterKind" {
							continue
						}
						if h.value < 0 {
							if len(lit.Elts) > baseLen {
								errs = append(errs, fmt.Errorf("%s: %s has no %s row to find its CtrXNames element by: regenerate it (KOP_ORDINALS_REGEN=1)", h.file, h.name, OrdinalsPath))
							}
							continue
						}
						i := int(h.value - charge)
						if i < baseLen || i >= len(lit.Elts) {
							continue
						}
						el := lit.Elts[i]
						if s, ok := el.(*ast.BasicLit); ok {
							if v, err := strconv.Unquote(s.Value); err == nil {
								h.forge = append(h.forge, v)
							}
						}
						cut[p] = append(cut[p], eltRange(src[p], lit, i, off))
					}
				}
			}
		}
	}
	for _, p := range kgenFiles {
		ast.Inspect(parsed[p], func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for i := 0; i+1 < len(call.Args); i++ {
				lit, ok := call.Args[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, _ := strconv.Unquote(lit.Value)
				name, ok := strings.CutPrefix(s, "kop.")
				if !ok || byName[name] == nil || !isKopSel(call.Args[i+1], name) {
					continue
				}
				end := off(call.Args[i+1].End())
				if i+2 < len(call.Args) {
					end = off(call.Args[i+2].Pos())
				} else if i > 0 {
					cut[p] = append(cut[p], [2]int{off(call.Args[i-1].End()), end})
					i++
					continue
				}
				cut[p] = append(cut[p], [2]int{off(lit.Pos()), end})
				i++
			}
			return true
		})
	}

	// Resolve each constant.
	var out []Adoption
	var news []*Entry
	rename := map[string]string{}
	for _, h := range hand {
		f := FamilyNamed(h.family)
		if e := t.entryNamedIn("kop", h.name); e != nil {
			if e.Family != h.family {
				errs = append(errs, fmt.Errorf("%s: %s is a %s by hand but %s is a %s", h.file, h.name, h.family, e.Path, e.Family))
				continue
			}
			out = append(out, Adoption{Name: h.name, Entry: e.Path, Case: AdoptSameName})
			continue
		}
		tail := strings.TrimPrefix(h.name, f.Prefix)
		var match []*Entry
		for _, e := range t.Entries {
			if e.Family != h.family {
				continue
			}
			hit := false
			for _, tok := range e.Forge {
				hit = hit || strings.EqualFold(goWords(tok), tail)
				for _, mine := range h.forge {
					hit = hit || strings.EqualFold(tok, mine)
				}
			}
			if hit {
				match = append(match, e)
			}
		}
		switch len(match) {
		case 0:
			e := &Entry{Path: Dir + "/" + h.family + "/" + h.name, Family: h.family, Name: h.name, Next: true,
				Forge: uniq(h.forge), Doc: h.doc, Status: Status{Kind: Reserved}}
			news = append(news, e)
			out = append(out, Adoption{Name: h.name, Entry: e.Path, Case: AdoptNext})
		case 1:
			rename[h.name] = match[0].Name
			out = append(out, Adoption{Name: h.name, Entry: match[0].Path, Case: AdoptRenamed})
		default:
			var ps []string
			for _, e := range match {
				ps = append(ps, e.Path)
			}
			errs = append(errs, fmt.Errorf("%s: %s matches %d entries (%s): adopt it by hand", h.file, h.name, len(match), strings.Join(ps, ", ")))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Write: the deletions, the renames, the new entries, the statuses.
	edited := map[string][]byte{}
	for _, p := range sortedKeys(cut) {
		b, err := deleteRanges(src[p], cut[p])
		if err != nil {
			return nil, fmt.Errorf("%s: after removing the hand constants: %v", p, err)
		}
		edited[p] = b
	}
	for _, p := range sortedKeys(edited) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), edited[p], 0o644); err != nil {
			return nil, err
		}
	}
	if err := renameUses(root, rename); err != nil {
		return nil, err
	}
	for _, e := range news {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(path.Dir(e.Path))), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(e.Path)), e.Format(), 0o644); err != nil {
			return nil, err
		}
	}
	used, err := kernelUses(root)
	if err != nil {
		return nil, err
	}
	t, err = Read(os.DirFS(root))
	if err != nil {
		return nil, err
	}
	for i := range out {
		e := t.Entry(out[i].Entry)
		out[i].Active = used[e.Name]
		if used[e.Name] && e.Status.Kind != Active {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(e.Path)), e.withStatus(Status{Kind: Active}), 0o644); err != nil {
				return nil, err
			}
		}
	}
	if _, err := WriteGen(root, false); err != nil {
		return out, err
	}
	return out, nil
}

// isGenFile reports whether p (a slash path) is one of the generated files.
func isGenFile(p string) bool {
	for _, g := range GenFiles {
		if p == g {
			return true
		}
	}
	return false
}

// registers is the hand constant a statement registers a name for
// (tbl[X] = "...", tbl[X-CtrCharge] = "..."), or nil.
func registers(st ast.Stmt, byName map[string]*handConst) *handConst {
	as, ok := st.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 {
		return nil
	}
	ix, ok := as.Lhs[0].(*ast.IndexExpr)
	if !ok {
		return nil
	}
	e := ix.Index
	if b, ok := e.(*ast.BinaryExpr); ok {
		e = b.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return byName[id.Name]
	}
	return nil
}

// regString is a registration's string value, or "".
func regString(st ast.Stmt) string {
	as := st.(*ast.AssignStmt)
	if lit, ok := as.Rhs[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		s, _ := strconv.Unquote(lit.Value)
		return s
	}
	return ""
}

func isKopSel(e ast.Expr, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "kop"
}

// specRange is the lines of spec i of g with its comments (gofmt puts a
// grouped spec on lines of its own): the whole declaration when it is the
// only spec.
func specRange(b []byte, g *ast.GenDecl, i int, off func(token.Pos) int) [2]int {
	vs := g.Specs[i].(*ast.ValueSpec)
	if len(g.Specs) == 1 {
		return declRange(b, g.Doc, g.Pos(), g.End(), off)
	}
	start, end := vs.Pos(), vs.End()
	if vs.Doc != nil {
		start = vs.Doc.Pos()
	}
	if vs.Comment != nil {
		end = vs.Comment.End()
	}
	return [2]int{lineStart(b, off(start)), lineEnd(b, off(end))}
}

// declRange is a declaration's bytes with its doc comment.
func declRange(b []byte, doc *ast.CommentGroup, pos, end token.Pos, off func(token.Pos) int) [2]int {
	if doc != nil {
		pos = doc.Pos()
	}
	return [2]int{lineStart(b, off(pos)), lineEnd(b, off(end))}
}

// eltRange is element i of lit with the separator before it (or after it,
// for the first element).
func eltRange(b []byte, lit *ast.CompositeLit, i int, off func(token.Pos) int) [2]int {
	el := lit.Elts[i]
	if i > 0 {
		return [2]int{off(lit.Elts[i-1].End()), off(el.End())}
	}
	if len(lit.Elts) > 1 {
		return [2]int{off(el.Pos()), off(lit.Elts[1].Pos())}
	}
	return [2]int{off(el.Pos()), off(el.End())}
}

func lineStart(b []byte, i int) int {
	for i > 0 && b[i-1] != '\n' {
		i--
	}
	return i
}

func lineEnd(b []byte, i int) int {
	for i < len(b) && b[i] != '\n' {
		i++
	}
	if i < len(b) {
		i++
	}
	return i
}

// deleteRanges deletes the ranges (merging overlaps) and gofmts the result.
func deleteRanges(b []byte, rs [][2]int) ([]byte, error) {
	sort.Slice(rs, func(i, j int) bool { return rs[i][0] < rs[j][0] })
	var out bytes.Buffer
	at := 0
	for _, r := range rs {
		if r[0] > at {
			out.Write(b[at:r[0]])
		}
		if r[1] > at {
			at = r[1]
		}
	}
	out.Write(b[at:])
	return format.Source(out.Bytes())
}

// gofmtPath is the gofmt of the toolchain that built this binary, else the
// one on PATH.
func gofmtPath() (string, error) {
	p := filepath.Join(runtime.GOROOT(), "bin", "gofmt")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return exec.LookPath("gofmt")
}

// renameUses rewrites every use of each old kop constant to its new name
// with gofmt -r: kop.Old in the Go files under dirac/ that name it, and
// the bare Old inside package kop.
func renameUses(root string, rename map[string]string) error {
	if len(rename) == 0 {
		return nil
	}
	gofmt, err := gofmtPath()
	if err != nil {
		return err
	}
	var files []string
	err = filepath.WalkDir(filepath.Join(root, "dirac"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	kop := filepath.Join(root, filepath.FromSlash(kopDir))
	for _, old := range sortedKeys(rename) {
		for _, p := range files {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if !bytes.Contains(b, []byte(old)) {
				continue
			}
			rule := "kop." + old + " -> kop." + rename[old]
			if filepath.Dir(p) == kop {
				rule = old + " -> " + rename[old]
			}
			if msg, err := exec.Command(gofmt, "-r", rule, "-w", p).CombinedOutput(); err != nil {
				return fmt.Errorf("gofmt -r %q %s: %v: %s", rule, p, err, msg)
			}
		}
	}
	return nil
}

// kernelUses is the kop constants package dirac's non-test files name
// (kop.X).
func kernelUses(root string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(root, "dirac", "*.go"))
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	fset := token.NewFileSet()
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") || isGenFile(path.Join("dirac", filepath.Base(p))) {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "kop" {
					used[sel.Sel.Name] = true
				}
			}
			return true
		})
	}
	return used, nil
}

// baseCtrXNamesLen is the number of elements of the CtrXNames literal at
// base (0 when base has none).
func baseCtrXNamesLen(root string, base *Tree) (int, error) {
	if base.commit == "" {
		return 0, nil
	}
	out, err := git(root, nil, "grep", "-l", "CtrXNames = ", base.commit, "--", kopDir+"/*.go")
	if err != nil {
		return 0, nil // no match: git grep exits 1
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_, p, _ := strings.Cut(line, ":")
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		b, err := git(root, nil, "show", base.commit+":"+p)
		if err != nil {
			return 0, err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, b, parser.SkipObjectResolution)
		if err != nil {
			return 0, err
		}
		n := -1
		ast.Inspect(f, func(x ast.Node) bool {
			vs, ok := x.(*ast.ValueSpec)
			if ok && len(vs.Names) == 1 && vs.Names[0].Name == "CtrXNames" && len(vs.Values) == 1 {
				if lit, ok := vs.Values[0].(*ast.CompositeLit); ok {
					n = len(lit.Elts)
				}
			}
			return n < 0
		})
		if n >= 0 {
			return n, nil
		}
	}
	return 0, nil
}

// withStatus is e's file with its status line set to s; every other byte
// is kept.
func (e *Entry) withStatus(s Status) []byte {
	var out bytes.Buffer
	for _, line := range strings.SplitAfter(string(e.raw), "\n") {
		if key, _, ok := strings.Cut(line, ":"); ok && key == "status" {
			nl := ""
			if strings.HasSuffix(line, "\n") {
				nl = "\n"
			}
			line = "status: " + s.String() + nl
		}
		out.WriteString(line)
	}
	return out.Bytes()
}

func uniq(ss []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
