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

// seatSensitiveGaps lists the mechanics of seat-sensitive-mechanics.tsv whose
// tests cell is `gap`: no test of this package pins them at four seats yet.
// It only shrinks: a mechanic leaves when its row names a four-seat test, and
// a new mechanic arrives with its test.
var seatSensitiveGaps = []string{
	"defending-player",
	"dethrone",
	"each-opponent",
	"extra-turns",
	"initiative",
	"melee",
	"monarch",
	"myriad",
}

const seatSensitiveTable = "testdata/seat-sensitive-mechanics.tsv"

type seatMechanicRow struct {
	mechanic string
	cr       []string
	pattern  *regexp.Regexp
	tests    []string // nil when gap
	gap      bool
	line     int
}

func readSeatSensitiveTable(t *testing.T) []seatMechanicRow {
	t.Helper()
	f, err := os.Open(seatSensitiveTable)
	if err != nil {
		t.Fatalf("open %s: %v", seatSensitiveTable, err)
	}
	defer f.Close()
	var rows []seatMechanicRow
	sc := bufio.NewScanner(f)
	ln := 0
	for sc.Scan() {
		ln++
		line := sc.Text()
		if ln == 1 {
			if line != "mechanic\tcr\tpattern\ttests" {
				t.Fatalf("%s:1: header %q, want mechanic\\tcr\\tpattern\\ttests", seatSensitiveTable, line)
			}
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			t.Fatalf("%s:%d: %d columns, want 4", seatSensitiveTable, ln, len(cols))
		}
		re, err := regexp.Compile(cols[2])
		if err != nil {
			t.Fatalf("%s:%d: pattern %q: %v", seatSensitiveTable, ln, cols[2], err)
		}
		r := seatMechanicRow{mechanic: cols[0], cr: strings.Split(cols[1], ","), pattern: re, line: ln}
		if cols[3] == "gap" {
			r.gap = true
		} else {
			r.tests = strings.Fields(cols[3])
			if len(r.tests) == 0 {
				t.Errorf("%s:%d: %s: empty tests cell (name a test or write gap)", seatSensitiveTable, ln, r.mechanic)
			}
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", seatSensitiveTable, err)
	}
	return rows
}

// packageTestSources parses every _test.go file of this package directory.
func packageTestSources(t *testing.T) (map[string]bool, map[string]bool) {
	t.Helper()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	all := map[string]bool{}
	four := map[string]bool{}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		tests, fours, err := fourSeatTestsInSource(p, src)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, n := range tests {
			all[n] = true
		}
		for _, n := range fours {
			four[n] = true
		}
	}
	return all, four
}

// fourSeatTests returns the test functions of this package that build a game
// of four seats (see fourSeatTestsInSource).
func fourSeatTests(t *testing.T) map[string]bool {
	t.Helper()
	_, four := packageTestSources(t)
	return four
}

var fourSeatPlayersRE = regexp.MustCompile(`"players": ?4\b`)

// fourSeatTestsInSource returns the Test functions of one file and the subset
// that builds a game of four seats: its body, or a function or package-level
// const/var of the same file it names (directly or through another such
// declaration), holds a "players":4 scenario, a
// newFixtureDeckSeats/commanderTaxGameSeats call whose seats argument is the
// literal 4, a crMultiplayerConfig call, a crResolutionEngine call with four
// slice arguments, or a Names: composite literal of four elements.
// Package-level declarations count because the scenario suites keep their
// "players":4 JSON in a const or var the test passes to its runner.
func fourSeatTestsInSource(name string, src []byte) (tests, fours []string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	decls := map[string]ast.Node{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Body != nil {
				decls[d.Name.Name] = d.Body
			}
		case *ast.GenDecl:
			if d.Tok != token.CONST && d.Tok != token.VAR {
				continue
			}
			for _, sp := range d.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if i < len(vs.Values) {
						decls[n.Name] = vs.Values[i]
					}
				}
			}
		}
	}
	direct := func(n ast.Node) bool {
		text := src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset]
		return fourSeatPlayersRE.Match(text) || fourSeatNode(n)
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Body == nil || !isTestFuncName(fd.Name.Name) {
			continue
		}
		tests = append(tests, fd.Name.Name)
		seen := map[string]bool{fd.Name.Name: true}
		work := []ast.Node{fd.Body}
		found := false
		for len(work) > 0 && !found {
			n := work[len(work)-1]
			work = work[:len(work)-1]
			if direct(n) {
				found = true
				break
			}
			for _, ref := range referencedNames(n) {
				if h, in := decls[ref]; in && !seen[ref] {
					seen[ref] = true
					work = append(work, h)
				}
			}
		}
		if found {
			fours = append(fours, fd.Name.Name)
		}
	}
	return tests, fours, nil
}

func isTestFuncName(n string) bool {
	if !strings.HasPrefix(n, "Test") || n == "TestMain" {
		return false
	}
	rest := n[len("Test"):]
	return rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
}

// referencedNames lists the bare identifiers under n, skipping selector
// fields (x.Name) and composite-literal keys, which never name a
// package-level declaration.
func referencedNames(n ast.Node) []string {
	skip := map[*ast.Ident]bool{}
	var out []string
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			skip[x.Sel] = true
		case *ast.KeyValueExpr:
			if k, ok := x.Key.(*ast.Ident); ok {
				skip[k] = true
			}
		case *ast.Ident:
			if !skip[x] {
				out = append(out, x.Name)
			}
		}
		return true
	})
	return out
}

func fourSeatNode(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch x := n.(type) {
		case *ast.CallExpr:
			id, ok := x.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			switch id.Name {
			case "newFixtureDeckSeats", "commanderTaxGameSeats":
				if len(x.Args) >= 3 {
					if lit, ok := x.Args[2].(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "4" {
						found = true
					}
				}
			case "crMultiplayerConfig":
				found = true
			case "crResolutionEngine":
				// t plus one deck-extras slice per seat.
				if len(x.Args) == 5 && x.Ellipsis == token.NoPos {
					found = true
				}
			}
		case *ast.KeyValueExpr:
			if k, ok := x.Key.(*ast.Ident); ok && k.Name == "Names" {
				if cl, ok := x.Value.(*ast.CompositeLit); ok && len(cl.Elts) == 4 {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func TestSeatSensitiveMechanicsHaveFourSeatTests(t *testing.T) {
	rows := readSeatSensitiveTable(t)
	all, four := packageTestSources(t)
	if len(four) == 0 {
		t.Fatal("no four-seat test detected in the package: detection is broken")
	}

	crIDs := map[string]bool{}
	if b, err := os.ReadFile("crcoverage/cr-index.tsv"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if id, _, ok := strings.Cut(line, "\t"); ok {
				crIDs[id] = true
			}
		}
	}

	allNames := make([]string, 0, len(all))
	for n := range all {
		allNames = append(allNames, n)
	}
	sort.Strings(allNames)

	inSlice := map[string]bool{}
	for _, m := range seatSensitiveGaps {
		inSlice[m] = true
	}
	seen := map[string]bool{}
	prev := ""
	withTests, gaps := 0, 0
	for _, r := range rows {
		where := seatSensitiveTable + ":" + strconv.Itoa(r.line) + ": " + r.mechanic
		if r.mechanic <= prev {
			t.Errorf("%s: rows not sorted by mechanic (after %s)", where, prev)
		}
		prev = r.mechanic
		seen[r.mechanic] = true
		if len(crIDs) > 0 {
			for _, id := range r.cr {
				if !crIDs[id] {
					t.Errorf("%s: cr %q is not an id of crcoverage/cr-index.tsv", where, id)
				}
			}
		}
		var cands []string
		for _, n := range allNames {
			if four[n] && r.pattern.MatchString(n) {
				cands = append(cands, n)
			}
		}
		t.Logf("candidates %s: %s", r.mechanic, strings.Join(cands, " "))
		if r.gap {
			gaps++
			if len(cands) > 0 {
				t.Errorf("%s: marked gap but four-seat test %s matches %q: name it in the row", where, cands[0], r.pattern)
			}
			if !inSlice[r.mechanic] {
				t.Errorf("%s: gap row not in seatSensitiveGaps (a new mechanic arrives with its four-seat test)", where)
			}
			continue
		}
		withTests++
		if inSlice[r.mechanic] {
			t.Errorf("%s: names tests but is still in seatSensitiveGaps: delete it from the slice", where)
		}
		for _, n := range r.tests {
			switch {
			case !all[n]:
				t.Errorf("%s: %s is not a test function of package rules", where, n)
			case !four[n]:
				t.Errorf("%s: %s does not build a game of four seats", where, n)
			}
		}
	}
	for _, m := range seatSensitiveGaps {
		if !seen[m] {
			t.Errorf("seatSensitiveGaps: %s has no row in %s: delete it", m, seatSensitiveTable)
		}
	}
	t.Logf("mechanics=%d with four-seat tests=%d gaps=%d", len(rows), withTests, gaps)
}

func TestFourSeatTestDetection(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"scenario players:4", "func TestX(t *testing.T) { run(t, `{\"players\":4}`) }", true},
		{"scenario players: 4", "func TestX(t *testing.T) { run(t, `{\"players\": 4}`) }", true},
		{"fixture seats 4", "func TestX(t *testing.T) { newFixtureDeckSeats(t, 1, 4, src) }", true},
		{"commander tax seats 4", "func TestX(t *testing.T) { commanderTaxGameSeats(t, 1, 4) }", true},
		{"multiplayer config", "func TestX(t *testing.T) { crMultiplayerConfig(t, reg, \"d\") }", true},
		{"resolution engine four slices", "func TestX(t *testing.T) { crResolutionEngine(t, nil, nil, nil, nil) }", true},
		{"names four", "func TestX(t *testing.T) { _ = Config{Names: []string{\"a\", \"b\", \"c\", \"d\"}} }", true},
		{"helper in file", "func TestX(t *testing.T) { pod(t) }\nfunc pod(t *testing.T) { _ = Config{Names: []string{\"a\", \"b\", \"c\", \"d\"}} }", true},
		{"helper const", "const pod = `{\"players\":4}`\nfunc TestX(t *testing.T) { run(t, pod) }", true},
		{"helper var via func", "func head() string { return `{\"players\": 4,` }\nvar pod = head() + `}`\nfunc TestX(t *testing.T) { run(t, pod) }", true},
		{"scenario players:3", "func TestX(t *testing.T) { run(t, `{\"players\":3}`) }", false},
		{"names two", "func TestX(t *testing.T) { _ = Config{Names: []string{\"a\", \"b\"}} }", false},
		{"fixture seats 3", "func TestX(t *testing.T) { newFixtureDeckSeats(t, 1, 3, src) }", false},
		{"helper never named", "const pod = `{\"players\":4}`\nfunc TestX(t *testing.T) { run(t, `{}`) }", false},
		{"resolution engine two slices", "func TestX(t *testing.T) { crResolutionEngine(t, nil, nil) }", false},
	}
	for _, c := range cases {
		tests, fours, err := fourSeatTestsInSource("x_test.go", []byte("package rules\n"+c.src+"\n"))
		if err != nil {
			t.Fatalf("%s: parse: %v", c.name, err)
		}
		if len(tests) != 1 || tests[0] != "TestX" {
			t.Fatalf("%s: tests=%v, want [TestX]", c.name, tests)
		}
		if got := len(fours) == 1; got != c.want {
			t.Errorf("%s: four-seat=%v, want %v", c.name, got, c.want)
		}
	}
}
