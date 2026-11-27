package rules

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A degrade site is a place in engine/effects or engine/rules that resolves
// an ability without the part it does not model and says so in a string
// literal ("unimplemented AssignGroup shape: ..."). Each site carries a
// stable code in testdata/degrade/sites.tsv so later work can count which
// cards reach it and shrink the list. The codes live in the table only: no
// Note text or event carries them.

// degradeRepoRoot is the repository root relative to this package's
// directory, where `go test` runs.
const degradeRepoRoot = "../.."

// degradeScanDirs are the slash paths, from the repository root, whose
// non-test sources the census scans recursively.
var degradeScanDirs = []string{"engine/effects", "engine/rules"}

const degradeTablePath = "testdata/degrade/sites.tsv"

const degradeTableHeader = "code\tfile\tfunc\tliteral\tkind"

// degradeLiteralPattern is what makes a string literal a degrade literal.
var degradeLiteralPattern = regexp.MustCompile(
	`(?i)\b(unsupported|unimplemented|unmodelled|unmodeled|not supported|not implemented|not modelled)\b`)

// degradeCodePattern is DG-<function name without dots>-<ordinal of the
// literal within that function>; a literal outside any function has the
// function name "-" (DG---01). A committed code never changes: a new literal
// takes its function's next free ordinal, and a renamed function keeps the
// codes of its rows (only the func cell changes).
var degradeCodePattern = regexp.MustCompile(`^DG-([A-Za-z0-9_]+|-)-[0-9]{2,}$`)

// degradeSite is one (enclosing function, literal) pair the scanner found.
type degradeSite struct {
	file    string // slash path from the repository root
	fn      string // FuncDecl name, Type.Method for a method, "-" outside any function
	literal string // unquoted text, tabs and newlines replaced by spaces
	kind    string // note, error or other
}

// degradeRow is one row of the committed table.
type degradeRow struct {
	code string
	degradeSite
	line int
}

func (s degradeSite) key() string { return s.file + "\x00" + s.fn + "\x00" + s.literal }

// degradeSourcesUnder lists the non-test .go files under dir, recursively,
// sorted; testdata and dot/underscore directories are skipped (the walk
// goSourcesUnder in internal/archtest uses).
func degradeSourcesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("degrade: walk %s: %v", dir, err)
	}
	sort.Strings(files)
	return files
}

// degradeSites scans every non-test source under degradeScanDirs and returns
// its degrade sites in file then source order, one per distinct
// (function, literal) pair.
func degradeSites(t *testing.T) []degradeSite {
	t.Helper()
	var sites []degradeSite
	for _, dir := range degradeScanDirs {
		for _, path := range degradeSourcesUnder(t, filepath.Join(degradeRepoRoot, filepath.FromSlash(dir))) {
			rel, err := filepath.Rel(degradeRepoRoot, path)
			if err != nil {
				t.Fatalf("degrade: rel %s: %v", path, err)
			}
			sites = append(sites, degradeSitesInFile(t, path, filepath.ToSlash(rel))...)
		}
	}
	return sites
}

func degradeSitesInFile(t *testing.T, path, rel string) []degradeSite {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("degrade: parse %s: %v", path, err)
	}
	var sites []degradeSite
	seen := map[string]bool{}
	for _, decl := range af.Decls {
		fn := "-"
		if fd, ok := decl.(*ast.FuncDecl); ok {
			fn = degradeFuncName(fd)
		}
		var stack []ast.Node
		ast.Inspect(decl, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil || !degradeLiteralPattern.MatchString(text) {
				return true
			}
			site := degradeSite{file: rel, fn: fn, literal: degradeFlatten(text), kind: degradeKind(stack)}
			if !seen[site.key()] {
				seen[site.key()] = true
				sites = append(sites, site)
			}
			return true
		})
	}
	return sites
}

// degradeFuncName is the FuncDecl's name, Type.Method for a method.
func degradeFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	typ := fd.Recv.List[0].Type
	for {
		switch x := typ.(type) {
		case *ast.StarExpr:
			typ = x.X
			continue
		case *ast.IndexExpr:
			typ = x.X
			continue
		case *ast.IndexListExpr:
			typ = x.X
			continue
		case *ast.Ident:
			return x.Name + "." + fd.Name.Name
		}
		return fd.Name.Name
	}
}

func degradeFlatten(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// degradeKind classifies a literal by its nearest classifying ancestor
// (stack ends with the literal itself): note for the Text: field of an
// events.Event literal or a call to a *Note function, error for
// fmt.Errorf, errors.New or a multi-value return, other otherwise.
func degradeKind(stack []ast.Node) string {
	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.KeyValueExpr:
			if key, ok := n.Key.(*ast.Ident); ok && key.Name == "Text" && i > 0 {
				if cl, ok := stack[i-1].(*ast.CompositeLit); ok && degradeIsEventsEvent(cl.Type) {
					return "note"
				}
			}
		case *ast.CallExpr:
			switch name := degradeCalleeName(n.Fun); {
			case strings.HasSuffix(name, "Note"):
				return "note"
			case name == "fmt.Errorf" || name == "errors.New":
				return "error"
			}
		case *ast.ReturnStmt:
			if len(n.Results) > 1 {
				return "error"
			}
		case *ast.FuncLit:
			return "other"
		}
	}
	return "other"
}

func degradeIsEventsEvent(typ ast.Expr) bool {
	sel, ok := typ.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "events" && sel.Sel.Name == "Event"
}

// degradeCalleeName is the callee written pkg.Name for a package-qualified
// identifier and the bare selector or identifier name otherwise.
func degradeCalleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && (pkg.Name == "fmt" || pkg.Name == "errors") {
			return pkg.Name + "." + f.Sel.Name
		}
		return f.Sel.Name
	case *ast.IndexExpr:
		return degradeCalleeName(f.X)
	}
	return ""
}

// readDegradeTable reads the committed table and checks its shape: the
// header, five cells a row, well-formed unique codes sorted ascending.
func readDegradeTable(t *testing.T) []degradeRow {
	t.Helper()
	f, err := os.Open(degradeTablePath)
	if err != nil {
		t.Fatalf("degrade: %v", err)
	}
	defer f.Close()
	var rows []degradeRow
	codes := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		if line == 1 {
			if sc.Text() != degradeTableHeader {
				t.Fatalf("degrade: %s header = %q, want %q", degradeTablePath, sc.Text(), degradeTableHeader)
			}
			continue
		}
		cells := strings.Split(sc.Text(), "\t")
		if len(cells) != 5 {
			t.Errorf("degrade: %s:%d has %d cells, want 5", degradeTablePath, line, len(cells))
			continue
		}
		row := degradeRow{code: cells[0], line: line,
			degradeSite: degradeSite{file: cells[1], fn: cells[2], literal: cells[3], kind: cells[4]}}
		if !degradeCodePattern.MatchString(row.code) {
			t.Errorf("degrade: %s:%d code %q is not DG-<func>-<NN>", degradeTablePath, line, row.code)
		}
		if codes[row.code] {
			t.Errorf("degrade: %s:%d code %s is used twice", degradeTablePath, line, row.code)
		}
		codes[row.code] = true
		if n := len(rows); n > 0 && rows[n-1].code >= row.code {
			t.Errorf("degrade: %s:%d code %s is not after %s: keep the table sorted by code",
				degradeTablePath, line, row.code, rows[n-1].code)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("degrade: read %s: %v", degradeTablePath, err)
	}
	return rows
}

func TestDegradeSitesAreListed(t *testing.T) {
	sites := degradeSites(t)
	rows := readDegradeTable(t)
	if len(sites) == 0 {
		t.Fatal("degrade: the scan found no degrade site; the scan roots are wrong")
	}

	byKey := map[string]*degradeRow{}
	byFuncLit := map[string][]*degradeRow{}
	for i := range rows {
		r := &rows[i]
		if byKey[r.key()] != nil {
			t.Errorf("degrade: rows %s and %s list the same site", byKey[r.key()].code, r.code)
			continue
		}
		byKey[r.key()] = r
		fl := r.fn + "\x00" + r.literal
		byFuncLit[fl] = append(byFuncLit[fl], r)
	}

	matched := map[string]bool{}
	for _, s := range sites {
		if r := byKey[s.key()]; r != nil {
			matched[r.code] = true
			if r.kind != s.kind {
				t.Errorf("degrade: row %s kind is %s, the literal in %s is %s: update the kind cell", r.code, r.kind, s.fn, s.kind)
			}
			continue
		}
		moved := false
		for _, r := range byFuncLit[s.fn+"\x00"+s.literal] {
			if !matched[r.code] && byKeyMissing(r, sites) {
				matched[r.code] = true
				moved = true
				t.Errorf("degrade: row %s: %s moved from %s to %s: update the file cell", r.code, s.fn, r.file, s.file)
				break
			}
		}
		if !moved {
			t.Errorf("degrade: new degrade site in %s: add a row with the next free code (%s\t%s\t%s\t%s)",
				s.fn, s.file, s.fn, s.literal, s.kind)
		}
	}
	for _, r := range rows {
		if !matched[r.code] {
			t.Errorf("degrade: row %s matches no literal: delete it (%s %s %q)", r.code, r.file, r.fn, r.literal)
		}
	}

	kinds := map[string]int{}
	perFunc := map[string]int{}
	for _, s := range sites {
		kinds[s.kind]++
		perFunc[s.file+" "+s.fn]++
	}
	t.Logf("degrade sites=%d note=%d error=%d other=%d", len(sites), kinds["note"], kinds["error"], kinds["other"])
	funcs := make([]string, 0, len(perFunc))
	for f := range perFunc {
		funcs = append(funcs, f)
	}
	sort.Slice(funcs, func(i, j int) bool {
		if perFunc[funcs[i]] != perFunc[funcs[j]] {
			return perFunc[funcs[i]] > perFunc[funcs[j]]
		}
		return funcs[i] < funcs[j]
	})
	for i := 0; i < len(funcs) && i < 10; i++ {
		t.Logf("  %3d %s", perFunc[funcs[i]], funcs[i])
	}
}

// byKeyMissing reports whether no scanned site still sits at r's own
// (file, func, literal): only then can r be the row of a moved site.
func byKeyMissing(r *degradeRow, sites []degradeSite) bool {
	for _, s := range sites {
		if s.key() == r.key() {
			return false
		}
	}
	return true
}

func TestDegradeLiteralPattern(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"unimplemented Blight shape: ", true},
		{" unimplemented", true}, // the literal part of "GainControl LoseControl$ "+x+" unimplemented"
		{"not supported", true},
		{"unmodelled ChoiceRestriction$ scope: ", true},
		{"Unmodeled X", true},
		{"feature not implemented", true},
		{"not modelled here", true},
		{"UNSUPPORTED", true},
		{"supported", false},
		{"implemented", false},
		{"modelled on", false},
		{"knownUnsupported", false},
		{"unsupportedness", false},
	} {
		if got := degradeLiteralPattern.MatchString(tc.text); got != tc.want {
			t.Errorf("degradeLiteralPattern.MatchString(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
