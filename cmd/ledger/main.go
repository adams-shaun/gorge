// Command ledger builds the judge-lane issue ledger the agent dashboard renders.
//
// The ledger is DERIVED, never hand-maintained, because a hand-copied list of
// open defects goes stale exactly the way AGENTS.md's own table header warns
// about: an entry is only as good as the thing it names still existing. Two
// sources, both measured:
//
//   - AGENTS.md's "Known approximations" table -> approved stand-ins. These are
//     dispositioned: someone looked at them and accepted them until a milestone.
//   - the opt-in conformance lane's -v output -> divergences. A FAIL leaf is an
//     UNFIXED defect with no approved row; that is the judge seat's deliverable.
//
// Usage:
//
//	GORGE_CR_CONFORMANCE=1 go test -count=1 ./rules -run TestCR -v > lane.txt
//	go run ./cmd/ledger -lane lane.txt -out .ds4/ledger.json
//
// -lane may be repeated for lanes in more than one package. With no -lane the
// ledger still carries the approximation rows, so the table is never blank.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Entry is one row of the ledger.
type Entry struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`   // "divergence" | "approximation"
	Status      string `json:"status"` // "open" | "closed" | "approved"
	Title       string `json:"title"`
	Where       string `json:"where"`
	Disposition string `json:"disposition"`
}

// Ledger is the whole document the dashboard reads.
type Ledger struct {
	Generated string         `json:"generated"`
	Commit    string         `json:"commit"`
	Counts    map[string]int `json:"counts"`
	Entries   []Entry        `json:"entries"`
}

func main() {
	var lanes multiFlag
	flag.Var(&lanes, "lane", "conformance lane `go test -v` output (repeatable)")
	out := flag.String("out", ".ds4/ledger.json", "where to write the ledger")
	root := flag.String("root", ".", "repo root")
	flag.Parse()

	docs, err := testDocs(*root)
	if err != nil {
		die(err)
	}
	var entries []Entry
	seen := map[string]bool{}
	for _, f := range lanes {
		b, err := os.ReadFile(f)
		if err != nil {
			die(err)
		}
		for _, e := range laneEntries(string(b), docs) {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			entries = append(entries, e)
		}
	}
	agents, err := os.ReadFile(filepath.Join(*root, "AGENTS.md"))
	if err != nil {
		die(err)
	}
	entries = append(entries, approximations(string(agents))...)

	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Status]++
	}
	l := Ledger{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Commit:    gitHead(*root),
		Counts:    counts,
		Entries:   entries,
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		die(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		die(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		die(err)
	}
	fmt.Printf("%s: %d entries (%d open, %d closed, %d approved)\n",
		*out, len(entries), counts["open"], counts["closed"], counts["approved"])
}

var resultLine = regexp.MustCompile(`^\s*--- (PASS|FAIL|SKIP): (\S+)`)

// laneEntries turns one lane's -v output into ledger rows, counting LEAVES
// only. A `--- PASS|FAIL: name` entry is a leaf iff no other reported name has
// it as a "/" prefix: a parent test that fails only because a subtest failed is
// the same defect counted twice, and double-counting a lane is how a lane stops
// meaning anything.
func laneEntries(s string, docs map[string]doc) []Entry {
	type res struct{ status, name string }
	var all []res
	names := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		if m := resultLine.FindStringSubmatch(line); m != nil {
			all = append(all, res{m[1], m[2]})
			names[m[2]] = true
		}
	}
	var out []Entry
	for _, r := range all {
		leaf := true
		for n := range names {
			if n != r.name && strings.HasPrefix(n, r.name+"/") {
				leaf = false
				break
			}
		}
		if !leaf || r.status == "SKIP" {
			continue
		}
		// A subtest's prose lives on the parent func; the subtest name is the
		// case, not a separate rule.
		fn := r.name
		if i := strings.Index(fn, "/"); i >= 0 {
			fn = fn[:i]
		}
		d := docs[fn]
		// The test NAME is the identity and the doc sentence is only support:
		// every subtest of one func shares that func's prose, so a table keyed
		// on the sentence shows three identical rows for three distinct cases.
		e := Entry{ID: r.name, Kind: "divergence", Title: d.title, Where: d.where}
		// Keep the disposition SHORT. The same 40-word paragraph repeated on
		// every open row is not information; the distinction between the three
		// statuses belongs in the section header, stated once.
		if r.status == "FAIL" {
			e.Status = "open"
			e.Disposition = "unfixed — no approved row"
		} else {
			e.Status = "closed"
			e.Disposition = "lane green — regression pin"
		}
		out = append(out, e)
	}
	return out
}

// approximations reads AGENTS.md's "Known approximations" table. Each row is an
// APPROVED stand-in with a named milestone, which is a different disposition
// from a lane failure and must never be merged with one on the page.
func approximations(s string) []Entry {
	lines := strings.Split(s, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "## Known approximations") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	var out []Entry
	n := 0
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "## ") {
			break
		}
		if !strings.HasPrefix(l, "| ") {
			continue
		}
		cells := splitRow(l)
		if len(cells) < 3 || cells[0] == "Stand-in" || strings.HasPrefix(cells[0], "---") {
			// A row with fewer than three cells is a MALFORMED table row, not a
			// row to quietly drop: it names no location and no milestone, which
			// is the one thing this table's own header forbids. Surface it.
			if strings.HasPrefix(l, "| ") && len(cells) < 3 && cells[0] != "Stand-in" &&
				!strings.HasPrefix(cells[0], "---") {
				n++
				out = append(out, Entry{
					ID:          fmt.Sprintf("approx-%02d", n),
					Kind:        "approximation",
					Status:      "open",
					Title:       cells[0],
					Where:       "AGENTS.md (malformed row)",
					Disposition: "malformed row — no Where, no milestone",
				})
			}
			continue
		}
		n++
		// Forge script fragments in the description carry their own "|"
		// separators, so the description is everything BUT the last two cells,
		// never cells[0]. Splitting naively put a script fragment in Where.
		title := strings.Join(cells[:len(cells)-2], " | ")
		out = append(out, Entry{
			ID:          fmt.Sprintf("approx-%02d", n),
			Kind:        "approximation",
			Status:      "approved",
			Title:       title,
			Where:       cells[len(cells)-2],
			Disposition: "approved — removed by " + cells[len(cells)-1],
		})
	}
	return out
}

func splitRow(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "|")
	l = strings.TrimSuffix(l, "|")
	parts := strings.Split(l, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

type doc struct{ title, where string }

// testDocs maps every Test func in the repo to its location and the first
// sentence of its doc comment. The doc comment is what the test's author said
// the rule IS, so it is a far better ledger title than the CamelCase name.
func testDocs(root string) (map[string]doc, error) {
	out := map[string]doc{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".worktrees", "node_modules", ".cards":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if perr != nil {
			return nil // a file that will not parse is not a reason to have no ledger
		}
		rel, _ := filepath.Rel(root, p)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			pos := fset.Position(fn.Pos())
			out[fn.Name.Name] = doc{
				title: firstSentence(fn.Doc.Text()),
				where: fmt.Sprintf("%s:%d", rel, pos.Line),
			}
		}
		return nil
	})
	return out, err
}

// firstSentence takes the lead sentence of a doc comment, dropping the Go
// convention's leading func name so the row reads as prose.
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	// "TestFoo checks that ..." -> "checks that ..."
	if i := strings.Index(s, " "); i > 0 && strings.HasPrefix(s[:i], "Test") {
		s = s[i+1:]
	}
	for i, r := range s {
		if r == '.' && (i+1 == len(s) || s[i+1] == ' ') {
			return s[:i+1]
		}
	}
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}

func gitHead(root string) string {
	b, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func die(err error) {
	fmt.Fprintln(os.Stderr, "ledger:", err)
	os.Exit(1)
}
