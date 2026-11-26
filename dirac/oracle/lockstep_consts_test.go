package oracle

// TestLockstepConstants pins dirac's numeric mirrors of gorge's constants:
// every dirac constant or assignment annotated with the lockstep tag
// (`<importpath>.<Ident>` anchor, below) must equal the gorge constant it
// names, so a gorge bump that skips the mirror fails here at commit time
// instead of hours later when a lockstep fuzz reaches the bound. The mirrors
// are plain source text on both sides (the dirac core imports nothing outside
// dirac), so the test parses both packages with go/parser and resolves the
// values with go/constant; it reads gorge's source, never gorge's engine.

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// wantLockstepConsts is the mirror census: the test fails when the annotation
// count moves, so a new or removed numeric mirror is noticed here instead of
// silently dropping out of the contract. 14 as of the 11-26 kernel
// maintainability audit.
const wantLockstepConsts = 14

// lockstepTag is the annotation a dirac mirror carries on the site's line (or
// the line above): the tag followed by the gorge anchor. The pieces are
// spelled apart so the mirror census (git grep of the tag over dirac) counts
// annotation sites, not this file's prose.
const lockstepTag = "//lockstep" + ":" + "const "

var lockstepAnchorRe = regexp.MustCompile(regexp.QuoteMeta(lockstepTag) + `([^\n]+)`)

// mirror is one annotated dirac site and the gorge constant it must equal.
type mirror struct {
	pkgDir string // absolute dir of the dirac package holding the site
	file   string // repo-relative, of the annotation
	line   int
	anchor string // <importpath>.<Ident>, optionally scaled (* 5 / 4)
	what   string // the dirac site's const name or printed assignment
	assign bool   // the site is an assignment (p.Step = 7), not a const
}

// constSpec is one package-level constant declaration, kept as source so the
// value resolves with iota and identifier references on demand.
type constSpec struct {
	expr  ast.Expr
	iota  int
	file  string // repo-relative
	line  int
	value constant.Value
	done  bool
	busy  bool
}

// constEnv holds the constants of one parsed package (a directory's non-test
// files) for value resolution.
type constEnv struct {
	fset  *token.FileSet
	root  string
	specs map[string]*constSpec
}

func TestLockstepConstants(t *testing.T) {
	root := repoRootDir()
	fset := token.NewFileSet()
	mirrors, err := collectMirrors(root, fset, filepath.Join(root, "dirac"))
	if err != nil {
		t.Fatalf("collect mirrors: %v", err)
	}
	if len(mirrors) != wantLockstepConsts {
		t.Fatalf("found %d annotated mirrors under dirac, want %d: annotate every new numeric mirror of a gorge constant (and update the census), or remove a stale one", len(mirrors), wantLockstepConsts)
	}
	sort.Slice(mirrors, func(i, j int) bool {
		if mirrors[i].file != mirrors[j].file {
			return mirrors[i].file < mirrors[j].file
		}
		return mirrors[i].line < mirrors[j].line
	})
	envs := map[string]*constEnv{}
	for _, m := range mirrors {
		dv, derr := m.diracValue(root, fset, envs)
		gv, gfile, gline, gerr := gorgeValue(root, fset, m.anchor, envs)
		if derr != nil {
			t.Errorf("%s:%d: %s: %v", m.file, m.line, m.what, derr)
			continue
		}
		if gerr != nil {
			t.Errorf("%s:%d: %s: %v", m.file, m.line, m.what, gerr)
			continue
		}
		if !constant.Compare(dv, token.EQL, gv) {
			t.Errorf("%s:%d: %s = %s diverges from gorge %s (%s:%d) = %s", m.file, m.line, m.what, dv, m.anchor, gfile, gline, gv)
		}
	}
}

// repoRootDir returns the module root: the nearest go.mod above the test
// binary's working directory (runtime.Caller is -trimpathed under the shim).
func repoRootDir() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			panic("lockstep consts: no go.mod above " + wd)
		}
	}
}

// collectMirrors walks dir for non-test Go files and returns every site
// annotated with the lockstep tag and a gorge anchor: a package-level const
// spec or a single-value assignment, with the annotation on the site's own
// line or the line above it.
func collectMirrors(root string, fset *token.FileSet, dir string) ([]mirror, error) {
	var out []mirror
	walk := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		ms, err := fileMirrors(root, fset, path)
		if err != nil {
			return fmt.Errorf("%s: %w", relPath(root, path), err)
		}
		out = append(out, ms...)
		return nil
	})
	if err := walk; err != nil {
		return nil, err
	}
	return out, nil
}

// fileMirrors parses one file and extracts its annotated sites.
func fileMirrors(root string, fset *token.FileSet, path string) ([]mirror, error) {
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	type site struct {
		spec   *ast.ValueSpec
		assign *ast.AssignStmt
	}
	sites := map[int]*site{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok {
				continue
			}
			sites[fset.Position(vs.Pos()).Line] = &site{spec: vs}
		}
	}
	for _, decl := range f.Decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			if a, ok := n.(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && len(a.Rhs) == 1 {
				sites[fset.Position(a.Pos()).Line] = &site{assign: a}
			}
			return true
		})
	}
	pkgDir := filepath.Dir(path)
	var out []mirror
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			mm := lockstepAnchorRe.FindStringSubmatch(c.Text)
			if mm == nil {
				continue
			}
			markLine := fset.Position(c.Pos()).Line
			s := sites[markLine+1]
			if s == nil {
				s = sites[markLine] // trailing on the site's own line
			}
			if s == nil {
				return nil, fmt.Errorf("%d: the annotation sits on no const or single-value assignment on the line above", markLine)
			}
			m := mirror{pkgDir: pkgDir, anchor: mm[1]}
			m.file, m.line = relPos(root, fset, c.Pos())
			switch {
			case s.spec != nil:
				if len(s.spec.Names) != 1 || len(s.spec.Values) != 1 {
					return nil, fmt.Errorf("%d: annotated const declares %d names with %d values; the annotation covers exactly one", markLine, len(s.spec.Names), len(s.spec.Values))
				}
				m.what = s.spec.Names[0].Name
			case s.assign != nil:
				var buf strings.Builder
				if err := printer.Fprint(&buf, fset, s.assign); err != nil {
					return nil, err
				}
				m.what, m.assign = buf.String(), true
			}
			out = append(out, m)
		}
	}
	return out, nil
}

// diracValue resolves the annotated dirac site's value: a package-level const
// through its package's declarations, an assignment through its expression.
func (m mirror) diracValue(root string, fset *token.FileSet, envs map[string]*constEnv) (constant.Value, error) {
	env, err := constEnvFor(root, fset, envs, m.pkgDir)
	if err != nil {
		return nil, err
	}
	if m.assign {
		return env.evalAssign(m, fset)
	}
	spec, ok := env.specs[m.what]
	if !ok {
		return nil, fmt.Errorf("%q is not a package-level constant of %s", m.what, filepath.Base(m.pkgDir))
	}
	return env.resolve(spec)
}

// evalAssign evaluates an annotated assignment's right-hand side (the
// realkernel `p.Step = 7` shape: a literal or const expression).
func (e *constEnv) evalAssign(m mirror, fset *token.FileSet) (constant.Value, error) {
	f, err := parser.ParseFile(fset, filepath.Join(e.root, m.file), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var target *ast.AssignStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && len(a.Rhs) == 1 {
			line := fset.Position(a.Pos()).Line
			if line == m.line+1 || line == m.line {
				target = a
			}
		}
		return true
	})
	if target == nil {
		return nil, fmt.Errorf("assignment at %s:%d not found on reparse", m.file, m.line)
	}
	return evalExpr(target.Rhs[0], 0, e)
}

// gorgeValue resolves the annotation's gorge side: `<importpath>.<Ident>`,
// optionally followed by integer scale factors (`* 5 / 4`, for a dirac
// constant that is a fixed multiple of the gorge one), read from that
// package's source under the module root.
func gorgeValue(root string, fset *token.FileSet, anchor string, envs map[string]*constEnv) (constant.Value, string, int, error) {
	fields := strings.Fields(anchor)
	ipath, ident := parseAnchor(fields[0])
	if ipath == "" {
		return nil, "", 0, fmt.Errorf("anchor %q is not <importpath>.<Ident>[ * n / n]", anchor)
	}
	env, err := constEnvFor(root, fset, envs, filepath.Join(root, filepath.FromSlash(ipath)))
	if err != nil {
		return nil, "", 0, err
	}
	spec, ok := env.specs[ident]
	if !ok {
		return nil, "", 0, fmt.Errorf("gorge %s declares no constant %q", ipath, ident)
	}
	v, err := env.resolve(spec)
	if err != nil {
		return nil, "", 0, err
	}
	for i := 1; i+1 < len(fields); i += 2 {
		fac, perr := strconv.Atoi(fields[i+1])
		if perr != nil || fac <= 0 {
			return nil, "", 0, fmt.Errorf("anchor %q: %q is not a positive integer factor", anchor, fields[i+1])
		}
		switch fields[i] {
		case "*":
			v = constant.BinaryOp(v, token.MUL, constant.MakeInt64(int64(fac)))
		case "/":
			v = constant.BinaryOp(v, token.QUO, constant.MakeInt64(int64(fac)))
		default:
			return nil, "", 0, fmt.Errorf("anchor %q: unsupported factor operation %q", anchor, fields[i])
		}
	}
	if len(fields)%2 == 0 {
		return nil, "", 0, fmt.Errorf("anchor %q: trailing %q has no factor", anchor, fields[len(fields)-1])
	}
	return v, spec.file, spec.line, nil
}

// parseAnchor splits `<importpath>.<Ident>` at the last dot.
func parseAnchor(a string) (ipath, ident string) {
	dot := strings.LastIndex(a, ".")
	if dot <= 0 {
		return "", ""
	}
	return a[:dot], a[dot+1:]
}

// constEnvFor parses (once) the non-test files of a package directory and
// indexes its package-level constants, with iota and implicit repetition.
func constEnvFor(root string, fset *token.FileSet, envs map[string]*constEnv, dir string) (*constEnv, error) {
	if env, ok := envs[dir]; ok {
		return env, nil
	}
	env := &constEnv{fset: fset, root: root, specs: map[string]*constSpec{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relPath(root, path), err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			var prev ast.Expr
			for i, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				expr := firstExpr(vs)
				if expr == nil {
					expr = prev // implicit repetition, with this spec's iota
				}
				spec := &constSpec{expr: expr, iota: i, file: relPath(root, path), line: fset.Position(vs.Pos()).Line}
				for _, nm := range vs.Names {
					env.specs[nm.Name] = spec
				}
				prev = expr
			}
		}
	}
	envs[dir] = env
	return env, nil
}

func firstExpr(vs *ast.ValueSpec) ast.Expr {
	if len(vs.Values) == 0 {
		return nil
	}
	return vs.Values[0]
}

// resolve evaluates a spec's expression, memoized, with cycle detection.
func (e *constEnv) resolve(s *constSpec) (constant.Value, error) {
	if s.done {
		return s.value, nil
	}
	if s.busy {
		return nil, fmt.Errorf("%s:%d: constant cycle", s.file, s.line)
	}
	s.busy = true
	v, err := evalExpr(s.expr, s.iota, e)
	s.busy = false
	if err != nil {
		return nil, fmt.Errorf("%s:%d: %w", s.file, s.line, err)
	}
	s.done, s.value = true, v
	return v, nil
}

// evalExpr evaluates a constant expression with go/constant: literals, iota,
// package constants (resolved in e), and the arithmetic the mirrored bounds
// use (+ - * / % and shifts).
func evalExpr(x ast.Expr, iota int, e *constEnv) (constant.Value, error) {
	switch x := x.(type) {
	case *ast.BasicLit:
		return constant.MakeFromLiteral(x.Value, x.Kind, 0), nil
	case *ast.Ident:
		switch x.Name {
		case "iota":
			return constant.MakeInt64(int64(iota)), nil
		case "true":
			return constant.MakeBool(true), nil
		case "false":
			return constant.MakeBool(false), nil
		}
		if e != nil {
			if s, ok := e.specs[x.Name]; ok {
				return e.resolve(s)
			}
		}
		return nil, fmt.Errorf("unresolved identifier %q", x.Name)
	case *ast.ParenExpr:
		return evalExpr(x.X, iota, e)
	case *ast.UnaryExpr:
		v, err := evalExpr(x.X, iota, e)
		if err != nil {
			return nil, err
		}
		return constant.UnaryOp(x.Op, v, 0), nil
	case *ast.BinaryExpr:
		xv, err := evalExpr(x.X, iota, e)
		if err != nil {
			return nil, err
		}
		yv, err := evalExpr(x.Y, iota, e)
		if err != nil {
			return nil, err
		}
		if x.Op == token.SHL || x.Op == token.SHR {
			yi, ok := constant.Int64Val(yv)
			if !ok {
				return nil, fmt.Errorf("shift amount %s is not an integer", yv)
			}
			return constant.Shift(xv, x.Op, uint(yi)), nil
		}
		return constant.BinaryOp(xv, x.Op, yv), nil
	}
	return nil, fmt.Errorf("unsupported constant expression %T", x)
}

func relPos(root string, fset *token.FileSet, pos token.Pos) (string, int) {
	p := fset.Position(pos)
	return relPath(root, p.Filename), p.Line
}

func relPath(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}
