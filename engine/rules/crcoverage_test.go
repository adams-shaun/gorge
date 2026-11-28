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

	"github.com/adams-shaun/gorge/engine/effects"
	_ "github.com/adams-shaun/gorge/engine/rules"
)

// The CR coverage tables (engine/rules/crcoverage/*.md) record, rule by
// rule, whether gorge follows the Comprehensive Rules and where the code that
// implements (or deviates from) each rule lives. TestCRCoverageIsCurrent keeps
// them honest: every row parses, every code reference names a file and a
// declaration that exist, every test reference names a test that exists, and
// the test logs the summary counts. A rename that orphans a reference fails
// here, so the table is fixed in the same change.
//
// The counts are not committed: every CR change would touch the same
// summary lines, so any two open changes would conflict at landing. Run this
// test with -v to read them. GORGE_UPDATE_CRCOVERAGE=1 deletes a stale summary
// block that an older branch still carries; references are never rewritten.

const (
	crCoverageDir       = "crcoverage"
	crCoverageUpdateEnv = "GORGE_UPDATE_CRCOVERAGE"
	crCoverageHeader    = "| rule | status | code | tests | notes |"
	crSummaryBegin      = "<!-- crcoverage:summary -->"
	crSummaryEnd        = "<!-- /crcoverage:summary -->"

	// cr-index.tsv lists every id of the pinned rules text (2026-08-07),
	// derived by scripts/cr_index.py; the two allow-lists only shrink.
	crIndexFile       = "cr-index.tsv"
	crIndexHeader     = "id\tkind\ttitle\tin_scope\texamples"
	crUnknownRowsFile = "cr-index-unknown-rows.txt"
	crUncoveredFile   = "cr-index-uncovered.txt"

	// deviates-untested.txt lists the rule cells of deviates rows that name
	// no test yet; it only shrinks.
	crDeviatesUntestedFile = "deviates-untested.txt"
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
	t.Parallel()
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
	untested := map[string]string{}
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
		if r.status == "deviates" && len(r.tests) == 0 {
			untested[r.rule] = fmt.Sprintf("%s: deviates needs a test that pins what the engine does", at)
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
	checkCRAllowList(t, crDeviatesUntestedFile, untested)
	checkCRSummary(t, rows)
	checkCRIndexCoverage(t, rows, readCRIndex(t))
	// status vs registry
	checkCRStatusVsRegistry(t, rows, readCRIndex(t), effects.Supported())
	// end status vs registry
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

// crIndexRow is one id of the Comprehensive Rules index. It carries no rules
// text: a title is a section title or a keyword name.
type crIndexRow struct {
	id, kind, title string
	inScope         bool
	examples        int
}

var (
	crIndexID    = regexp.MustCompile(`^\d{3}(\.\d+[a-z]*)?$`)
	crIndexKinds = map[string]*regexp.Regexp{
		"section": regexp.MustCompile(`^\d{3}$`),
		"rule":    regexp.MustCompile(`^\d{3}\.\d+$`),
		"subrule": regexp.MustCompile(`^\d{3}\.\d+[a-z]+$`),
	}
)

// parseCRIndex reads the index and returns its rows plus every way it is
// malformed.
func parseCRIndex(data string) ([]crIndexRow, []string) {
	var rows []crIndexRow
	var bad []string
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	if lines[0] != crIndexHeader {
		bad = append(bad, fmt.Sprintf("header is %q, want %q", lines[0], crIndexHeader))
	}
	seen := map[string]bool{}
	for n, line := range lines[1:] {
		at := fmt.Sprintf("%s:%d", crIndexFile, n+2)
		cells := strings.Split(line, "\t")
		if len(cells) != 5 {
			bad = append(bad, fmt.Sprintf("%s: want 5 columns, got %d", at, len(cells)))
			continue
		}
		for _, c := range cells {
			if len(c) > 80 {
				bad = append(bad, fmt.Sprintf("%s: a cell is %d characters; the index holds no rules text", at, len(c)))
			}
		}
		r := crIndexRow{id: cells[0], kind: cells[1], title: cells[2]}
		if re, ok := crIndexKinds[r.kind]; !ok || !re.MatchString(r.id) {
			bad = append(bad, fmt.Sprintf("%s: id %q is not a %q (kinds: section, rule, subrule)", at, r.id, r.kind))
		}
		if seen[r.id] {
			bad = append(bad, fmt.Sprintf("%s: id %s repeats", at, r.id))
		}
		seen[r.id] = true
		if r.kind == "section" && r.title == "" {
			bad = append(bad, fmt.Sprintf("%s: section %s has no title", at, r.id))
		}
		switch cells[3] {
		case "0", "1":
			r.inScope = cells[3] == "1"
		default:
			bad = append(bad, fmt.Sprintf("%s: in_scope %q is not 0 or 1", at, cells[3]))
		}
		if _, err := fmt.Sscanf(cells[4], "%d", &r.examples); err != nil || fmt.Sprint(r.examples) != cells[4] {
			bad = append(bad, fmt.Sprintf("%s: examples %q is not a count", at, cells[4]))
		}
		rows = append(rows, r)
	}
	return rows, bad
}

func readCRIndex(t *testing.T) []crIndexRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crCoverageDir, crIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	rows, bad := parseCRIndex(string(data))
	if len(bad) > 0 {
		t.Fatalf("%s is malformed (see TestCRIndexIsWellFormed): %s", crIndexFile, bad[0])
	}
	return rows
}

func TestCRIndexIsWellFormed(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(crCoverageDir, crIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	rows, bad := parseCRIndex(string(data))
	for _, b := range bad {
		t.Error(b)
	}
	if len(rows) == 0 {
		t.Fatalf("%s has no rows", crIndexFile)
	}
	// The negative cases: each malformed index is refused.
	for name, text := range map[string]string{
		"header":   "id\tkind\n100\tsection\tGeneral\t1\t0\n",
		"columns":  crIndexHeader + "\n100\tsection\tGeneral\t1\n",
		"kind":     crIndexHeader + "\n100.1\tsection\tGeneral\t1\t0\n",
		"repeat":   crIndexHeader + "\n100\tsection\tGeneral\t1\t0\n100\tsection\tGeneral\t1\t0\n",
		"untitled": crIndexHeader + "\n100\tsection\t\t1\t0\n",
		"in_scope": crIndexHeader + "\n100\tsection\tGeneral\t2\t0\n",
		"examples": crIndexHeader + "\n100\tsection\tGeneral\t1\tx\n",
		"text":     crIndexHeader + "\n100.1\trule\t" + strings.Repeat("x", 81) + "\t1\t0\n",
	} {
		if _, bad := parseCRIndex(text); len(bad) == 0 {
			t.Errorf("%s: a malformed index passed", name)
		}
	}
}

// crParentRule is the rule a subrule belongs to; a rule or section is its own.
func crParentRule(id string) string {
	return strings.TrimRight(id, "abcdefghijklmnopqrstuvwxyz")
}

// crUnder reports whether id lies inside anc: a rule or subrule of a section,
// or a subrule of a rule.
func crUnder(id, anc string) bool {
	if len(id) <= len(anc) || !strings.HasPrefix(id, anc) {
		return false
	}
	c := id[len(anc)]
	if len(anc) == 3 {
		return c == '.'
	}
	return strings.Contains(anc, ".") && c >= 'a' && c <= 'z'
}

// crExpandID completes an abbreviated part of a rule cell from the id before
// it: `d` after 702.1a is 702.1d, `3` after 100.2 is 100.3.
func crExpandID(part, prev string) string {
	switch {
	case strings.Contains(part, "."), prev == "", !strings.Contains(prev, "."):
		return part
	case strings.Trim(part, "abcdefghijklmnopqrstuvwxyz") == "":
		return crParentRule(prev) + part
	}
	return prev[:3] + "." + part
}

// crRowIDs expands a row's rule cell to the index ids it names, in cell
// order. A range a-b covers every index id from a to b in index order plus
// what lies under b. Parts the index lacks come back in unknown.
func crRowIDs(rule string, index []crIndexRow) (ids []string, unknown []string) {
	pos := make(map[string]int, len(index))
	for i, r := range index {
		pos[r.id] = i
	}
	prev := ""
	for _, part := range strings.Split(rule, ",") {
		a, b, isRange := strings.Cut(part, "-")
		a = crExpandID(a, prev)
		prev = a
		if !isRange {
			if _, ok := pos[a]; ok {
				ids = append(ids, a)
			} else {
				unknown = append(unknown, a)
			}
			continue
		}
		b = crExpandID(b, a)
		prev = b
		i, okA := pos[a]
		j, okB := pos[b]
		if !okA {
			unknown = append(unknown, a)
		}
		if !okB {
			unknown = append(unknown, b)
		}
		if !okA || !okB {
			continue
		}
		if j < i {
			unknown = append(unknown, a+"-"+b)
			continue
		}
		for k := i; k <= j; k++ {
			ids = append(ids, index[k].id)
		}
		for k := j + 1; k < len(index) && crUnder(index[k].id, b); k++ {
			ids = append(ids, index[k].id)
		}
	}
	return ids, unknown
}

// crUncovered lists, in index order, the rule and subrule ids no listed id
// covers. An id is covered when it is listed, or its parent rule is, or its
// section is, or (for a rule) one of its subrules is.
func crUncovered(listed map[string]bool, index []crIndexRow) []string {
	hasSub := map[string]bool{}
	for id := range listed {
		if p := crParentRule(id); p != id {
			hasSub[p] = true
		}
	}
	var out []string
	for _, r := range index {
		if r.kind == "section" {
			continue
		}
		if listed[r.id] || listed[crParentRule(r.id)] || listed[r.id[:3]] || (r.kind == "rule" && hasSub[r.id]) {
			continue
		}
		out = append(out, r.id)
	}
	return out
}

// crIDLess orders ids as the rules do: 113.9 before 113.10, 704.5z before
// 704.5aa.
func crIDLess(a, b string) bool {
	key := func(id string) (string, int, int, string) {
		rule := crParentRule(id)
		n := -1
		if len(rule) > 4 {
			fmt.Sscanf(rule[4:], "%d", &n)
		}
		return id[:min(3, len(id))], n, len(id) - len(rule), id[len(rule):]
	}
	s1, n1, l1, x1 := key(a)
	s2, n2, l2, x2 := key(b)
	switch {
	case s1 != s2:
		return s1 < s2
	case n1 != n2:
		return n1 < n2
	case l1 != l2:
		return l1 < l2
	}
	return x1 < x2
}

// checkCRAllowList fails each id in got that the allow-list lacks, and each
// allow-list line that no longer fails (the list only shrinks).
func checkCRAllowList(t *testing.T, name string, got map[string]string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crCoverageDir, name))
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	var lines []string
	for _, l := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if l == "" {
			continue
		}
		if allowed[l] {
			t.Errorf("%s: %s is listed twice", name, l)
		}
		allowed[l] = true
		lines = append(lines, l)
	}
	if !sort.SliceIsSorted(lines, func(i, j int) bool { return crIDLess(lines[i], lines[j]) }) {
		t.Errorf("%s: lines are not in rules order", name)
	}
	for _, l := range lines {
		if _, ok := got[l]; !ok {
			t.Errorf("%s lists %s, which no longer fails: delete the line", name, l)
		}
	}
	ids := make([]string, 0, len(got))
	for id := range got {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return crIDLess(ids[i], ids[j]) })
	for _, id := range ids {
		if !allowed[id] {
			t.Errorf("%s", got[id])
		}
	}
}

// checkCRIndexCoverage holds the tables to the rules index: every rule and
// subrule id has a row and every row names real ids, apart from the two
// shrink-only allow-lists. It logs the summary over in-scope ids.
func checkCRIndexCoverage(t *testing.T, rows []crRow, index []crIndexRow) {
	t.Helper()
	inScope := map[string]bool{}
	for _, r := range index {
		inScope[r.id] = r.inScope
	}
	listed := map[string]bool{}
	unknown := map[string]string{}
	var scoped []crRow
	for _, r := range rows {
		ids, bad := crRowIDs(r.rule, index)
		for _, id := range bad {
			if _, dup := unknown[id]; !dup {
				unknown[id] = fmt.Sprintf("%s:%d (%s): names %s, which the CR index (%s) lacks", r.file, r.line, r.rule, id, crIndexFile)
			}
		}
		for _, id := range ids {
			listed[id] = true
			if inScope[id] {
				s := r
				s.rule = id
				scoped = append(scoped, s)
			}
		}
	}
	uncovered := map[string]string{}
	for _, id := range crUncovered(listed, index) {
		uncovered[id] = fmt.Sprintf("CR %s has no coverage row (add one, or list it in %s)", id, crUncoveredFile)
	}
	checkCRAllowList(t, crUnknownRowsFile, unknown)
	checkCRAllowList(t, crUncoveredFile, uncovered)
	t.Logf("CR coverage summary over in-scope ids:%s", renderCRSummary(scoped))
}

func TestCRRowIDs(t *testing.T) {
	t.Parallel()
	var index []crIndexRow
	for _, id := range []string{
		"100", "100.1", "100.2", "100.2a", "100.3", "100.3a", "100.3b", "100.4",
		"702", "702.1", "702.1a", "702.1b", "702.1c", "702.1d",
		"800", "800.4", "800.4a", "800.4b", "800.4c", "800.4d",
	} {
		kind := "subrule"
		switch {
		case len(id) == 3:
			kind = "section"
		case crParentRule(id) == id:
			kind = "rule"
		}
		index = append(index, crIndexRow{id: id, kind: kind, inScope: true})
	}
	for _, c := range []struct {
		rule         string
		ids, unknown []string
	}{
		{rule: "702.1a,d", ids: []string{"702.1a", "702.1d"}},
		{rule: "800.4a-c", ids: []string{"800.4a", "800.4b", "800.4c"}},
		{rule: "100.2-100.3", ids: []string{"100.2", "100.2a", "100.3", "100.3a", "100.3b"}},
		{rule: "100.2-3", ids: []string{"100.2", "100.2a", "100.3", "100.3a", "100.3b"}},
		{rule: "800.4,800.4e", ids: []string{"800.4"}, unknown: []string{"800.4e"}},
		{rule: "100.9-100.10", unknown: []string{"100.9", "100.10"}},
	} {
		ids, unknown := crRowIDs(c.rule, index)
		if fmt.Sprint(ids) != fmt.Sprint(c.ids) || fmt.Sprint(unknown) != fmt.Sprint(c.unknown) {
			t.Errorf("crRowIDs(%q) = %v, %v; want %v, %v", c.rule, ids, unknown, c.ids, c.unknown)
		}
	}
	for _, c := range []struct {
		name      string
		listed    []string
		uncovered string
	}{
		// 800.4 covers its subrules; 100.1 and 100.4 have nothing.
		{"a parent row covers its subrules", []string{"100.2", "100.3", "702", "800.4"}, "[100.1 100.4]"},
		// 100.3a covers its heading rule 100.3 but not its sibling 100.3b.
		{"a subrule row covers its heading rule", []string{"100.1", "100.2", "100.3a", "100.4", "702", "800"}, "[100.3b]"},
		{"a section row covers everything under it", []string{"100", "702", "800"}, "[]"},
	} {
		listed := map[string]bool{}
		for _, id := range c.listed {
			listed[id] = true
		}
		if got := fmt.Sprint(crUncovered(listed, index)); got != c.uncovered {
			t.Errorf("%s: uncovered %s, want %s", c.name, got, c.uncovered)
		}
	}
	if !crIDLess("113.9", "113.10") || !crIDLess("704.5z", "704.5aa") || !crIDLess("113", "113.1") || crIDLess("606.5", "506.8") {
		t.Error("crIDLess does not follow rules order")
	}
}

// status vs registry

// checkCRStatusVsRegistry refuses a not-impl row of section 702 that covers a
// keyword's heading rule (702.N) or its first subrule (702.Na) while the
// effects registry holds the kw: entry for that keyword. A row listing only
// later subrules is exempt: one unimplemented subrule of a registered keyword
// is a legitimate not-impl.
func checkCRStatusVsRegistry(t *testing.T, rows []crRow, index []crIndexRow, supported map[string]bool) {
	t.Helper()
	titles := make(map[string]string, len(index))
	for _, r := range index {
		if r.kind == "rule" {
			titles[r.id] = r.title
		}
	}
	for _, r := range rows {
		if r.status != "not-impl" {
			continue
		}
		ids, _ := crRowIDs(r.rule, index)
		if len(ids) == 0 || !strings.HasPrefix(ids[0], "702.") {
			continue
		}
		at := fmt.Sprintf("%s:%d (%s)", r.file, r.line, r.rule)
		reported := map[string]bool{}
		for _, id := range ids {
			head := crParentRule(id)
			if !strings.HasPrefix(head, "702.") || (id != head && id != head+"a") || reported[head] {
				continue
			}
			title := titles[head]
			if title == "" || !crRegisteredKeyword(title, supported) {
				continue
			}
			reported[head] = true
			t.Errorf("%s: not-impl, but kw:%s is registered; re-audit the row", at, title)
		}
	}
}

// crFold lower-cases s and drops every character outside a-z0-9, so the
// registry's `kw:Split second` and the index's `Split Second` compare equal.
func crFold(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// crRegisteredKeyword reports whether supported holds a kw: entry whose name
// folds to the folded title.
func crRegisteredKeyword(title string, supported map[string]bool) bool {
	want := crFold(title)
	if want == "" {
		return false
	}
	for k := range supported {
		if name, ok := strings.CutPrefix(k, "kw:"); ok && crFold(name) == want {
			return true
		}
	}
	return false
}

func TestCRRegisteredKeyword(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		title     string
		supported map[string]bool
		want      bool
	}{
		{"Split Second", map[string]bool{"kw:Split second": true}, true},
		{"For Mirrodin!", map[string]bool{"kw:For Mirrodin": true}, true},
		{"Living Metal", map[string]bool{"kw:Living metal": true}, true},
		{"Banding", map[string]bool{"kw:Bands with other": true}, false},
		{"Flying", map[string]bool{"api:Flying": true}, false},
	} {
		if got := crRegisteredKeyword(c.title, c.supported); got != c.want {
			t.Errorf("crRegisteredKeyword(%q, %v) = %v, want %v", c.title, c.supported, got, c.want)
		}
	}
}
