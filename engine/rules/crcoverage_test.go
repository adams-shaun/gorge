package rules_test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The CR coverage tables (engine/rules/crcoverage/*.md) record, rule by
// rule, whether gorge follows the Comprehensive Rules and where the code that
// implements (or deviates from) each rule lives. TestCRCoverageIsCurrent keeps
// them honest: every row parses, every code reference names a file and a
// declaration that exist, every test reference names a test that exists, and
// the test logs the summary counts. A rename that orphans a reference fails
// here, so the table is fixed in the same change.
//
// The counts are not committed (2026-10-21): every CR ticket changed the same
// summary lines, so any two open CR tickets conflicted at landing. Run this
// test with -v to read them. GORGE_UPDATE_CRCOVERAGE=1 deletes a stale summary
// block that an older branch still carries; references are never rewritten.

const (
	crCoverageDir       = "crcoverage"
	crCoverageUpdateEnv = "GORGE_UPDATE_CRCOVERAGE"
	crCoverageHeader    = "| rule | status | code | tests | notes |"
	crSummaryBegin      = "<!-- crcoverage:summary -->"
	crSummaryEnd        = "<!-- /crcoverage:summary -->"
)

// crStatuses is the closed status vocabulary, in summary-column order.
var crStatuses = []string{"follows", "partial", "deviates", "not-impl", "definition", "n/a", "unknown"}

var (
	crRuleID  = regexp.MustCompile(`^\d{3}(\.\d+[a-z]*)?([-,/](\d{3}(\.\d+[a-z]*)?|\d+[a-z]*|[a-z]))*$`)
	crRef     = regexp.MustCompile("^`([^`#]+)(?:#([A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)?))?`$")
	crTestRef = regexp.MustCompile("^`((?:Test|Benchmark|Fuzz|Example)[A-Za-z0-9_]*)`$")
	crTestDef = regexp.MustCompile(`(?m)^func ((?:Test|Benchmark|Fuzz|Example)[A-Za-z0-9_]*)\(`)
)

type crRow struct {
	file, rule, status string
	line               int
	code, tests        []string
}

func TestCRCoverageIsCurrent(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root not at %s: %v", root, err)
	}
	files, err := filepath.Glob(filepath.Join(crCoverageDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []crRow
	for _, f := range files {
		if filepath.Base(f) == "README.md" {
			continue
		}
		rows = append(rows, readCRTable(t, f)...)
	}
	if len(rows) == 0 {
		t.Fatalf("no CR coverage rows under %s", crCoverageDir)
	}

	decls := map[string]map[string]bool{}
	tests := crTestNames(t, root)
	seen := map[string]string{}
	for _, r := range rows {
		at := fmt.Sprintf("%s:%d (%s)", r.file, r.line, r.rule)
		if !crRuleID.MatchString(r.rule) {
			t.Errorf("%s: malformed rule id %q", at, r.rule)
		}
		if prev, dup := seen[r.rule]; dup {
			t.Errorf("%s: rule %s already has a row at %s", at, r.rule, prev)
		}
		seen[r.rule] = at
		if !crKnownStatus(r.status) {
			t.Errorf("%s: status %q is not one of %v", at, r.status, crStatuses)
		}
		if (r.status == "follows" || r.status == "partial" || r.status == "deviates") && len(r.code) == 0 {
			t.Errorf("%s: status %s needs at least one code reference", at, r.status)
		}
		for _, c := range r.code {
			m := crRef.FindStringSubmatch(c)
			if m == nil {
				t.Errorf("%s: code reference %s is not `path` or `path#Symbol`", at, c)
				continue
			}
			path, sym := m[1], m[2]
			full := filepath.Join(root, filepath.FromSlash(path))
			info, err := os.Stat(full)
			if err != nil {
				t.Errorf("%s: %s does not exist", at, path)
				continue
			}
			if sym == "" {
				if info.IsDir() {
					t.Errorf("%s: %s is a directory, not a file", at, path)
				}
				continue
			}
			if !strings.HasSuffix(path, ".go") {
				t.Errorf("%s: %s: symbols are only checked in Go files", at, c)
				continue
			}
			d, ok := decls[path]
			if !ok {
				d = crDecls(t, full)
				decls[path] = d
			}
			if !d[sym] {
				t.Errorf("%s: %s declares no %s", at, path, sym)
			}
		}
		for _, tr := range r.tests {
			m := crTestRef.FindStringSubmatch(tr)
			if m == nil {
				t.Errorf("%s: test reference %s is not a backticked test name", at, tr)
				continue
			}
			if !tests[m[1]] {
				t.Errorf("%s: no test named %s in the module", at, m[1])
			}
		}
	}
	checkCRSummary(t, rows)
}

func crKnownStatus(s string) bool {
	for _, k := range crStatuses {
		if s == k {
			return true
		}
	}
	return false
}

// readCRTable parses the one table in a coverage file.
func readCRTable(t *testing.T, path string) []crRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []crRow
	inTable := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == crCoverageHeader:
			if inTable || len(rows) > 0 {
				t.Errorf("%s:%d: a coverage file holds exactly one table", path, n)
			}
			inTable = true
			continue
		case !inTable:
			continue
		case !strings.HasPrefix(line, "|"):
			inTable = false
			continue
		case strings.HasPrefix(line, "|---") || strings.HasPrefix(line, "| ---"):
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 5 {
			t.Errorf("%s:%d: want 5 cells, got %d (notes must not contain |)", path, n, len(cells))
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, crRow{
			file: filepath.Base(path), line: n,
			rule: cells[0], status: cells[1],
			code: crFields(cells[2]), tests: crFields(cells[3]),
		})
	}
	if len(rows) == 0 {
		t.Errorf("%s: no table with header %q", path, crCoverageHeader)
	}
	return rows
}

func crFields(cell string) []string {
	if cell == "-" || cell == "" {
		return nil
	}
	return strings.Fields(cell)
}

// crDecls lists the declarations of one Go file the way references name
// them: funcs, types, vars and consts by name; methods and struct fields as
// Type.Name.
func crDecls(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Errorf("parse %s: %v", path, err)
		return nil
	}
	d := map[string]bool{}
	for _, decl := range f.Decls {
		switch dd := decl.(type) {
		case *ast.FuncDecl:
			if dd.Recv != nil && len(dd.Recv.List) > 0 {
				d[crRecvName(dd.Recv.List[0].Type)+"."+dd.Name.Name] = true
			} else {
				d[dd.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, sp := range dd.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					d[s.Name.Name] = true
					if st, ok := s.Type.(*ast.StructType); ok {
						for _, fl := range st.Fields.List {
							for _, n := range fl.Names {
								d[s.Name.Name+"."+n.Name] = true
							}
						}
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						d[n.Name] = true
					}
				}
			}
		}
	}
	return d
}

func crRecvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return crRecvName(t.X)
	case *ast.IndexExpr:
		return crRecvName(t.X)
	case *ast.IndexListExpr:
		return crRecvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// crTestNames collects every test, benchmark, fuzz and example function name
// declared in the module's _test.go files.
func crTestNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if b := d.Name(); p != root && (strings.HasPrefix(b, ".") || b == "testdata" || b == "node_modules" || b == "webdist") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range crTestDef.FindAllSubmatch(data, -1) {
			names[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// checkCRSummary logs the tables' counts. README.md must not carry the old
// generated summary block: a branch from before 2026-10-21 still does, and
// GORGE_UPDATE_CRCOVERAGE=1 deletes it.
func checkCRSummary(t *testing.T, rows []crRow) {
	t.Helper()
	t.Logf("CR coverage summary:%s", renderCRSummary(rows))
	readme := filepath.Join(crCoverageDir, "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	b, e := strings.Index(text, crSummaryBegin), strings.Index(text, crSummaryEnd)
	if b < 0 && e < 0 {
		return
	}
	if b < 0 || e < b {
		t.Fatalf("%s: unbalanced %s ... %s markers", readme, crSummaryBegin, crSummaryEnd)
	}
	if os.Getenv(crCoverageUpdateEnv) == "1" {
		text = text[:b] + text[e+len(crSummaryEnd):]
		if err := os.WriteFile(readme, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("deleted the %s summary block", readme)
		return
	}
	t.Errorf("%s carries the old generated summary block; the counts are no longer committed. Delete it (rerun with %s=1)", readme, crCoverageUpdateEnv)
}

func renderCRSummary(rows []crRow) string {
	counts := map[string]map[string]int{}
	var order []string
	for _, r := range rows {
		if counts[r.file] == nil {
			counts[r.file] = map[string]int{}
			order = append(order, r.file)
		}
		counts[r.file][r.status]++
	}
	sort.Strings(order)
	var sb strings.Builder
	sb.WriteString("\n| file | rows | " + strings.Join(crStatuses, " | ") + " |\n|---|---:|")
	sb.WriteString(strings.Repeat("---:|", len(crStatuses)) + "\n")
	total := map[string]int{}
	for _, f := range order {
		n := 0
		cells := make([]string, len(crStatuses))
		for i, s := range crStatuses {
			c := counts[f][s]
			n += c
			total[s] += c
			cells[i] = fmt.Sprint(c)
		}
		fmt.Fprintf(&sb, "| [%s](%s) | %d | %s |\n", strings.TrimSuffix(f, ".md"), f, n, strings.Join(cells, " | "))
	}
	cells := make([]string, len(crStatuses))
	for i, s := range crStatuses {
		cells[i] = fmt.Sprint(total[s])
	}
	fmt.Fprintf(&sb, "| **all** | %d | %s |\n", len(rows), strings.Join(cells, " | "))
	return sb.String()
}
