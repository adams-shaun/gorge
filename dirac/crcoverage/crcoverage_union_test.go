package crcoverage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A merge=union (.gitattributes) of two branches that edited the same rule row
// leaves both versions of the row in one chapter file. Update mode reconciles
// them before validation; outside update mode the duplicate stays an error so
// the gate catches an unreconciled tree.

// crReadChapter reads one chapter file, first collapsing merge=union duplicate
// rows when KERNEL_UPDATE_CRCOVERAGE=1. Every reader of a chapter file goes
// through here, so a future validation gets the reconciliation too.
func crReadChapter(t *testing.T, path string) []crRow {
	t.Helper()
	crDedupeUnionRows(t, path)
	return readCRTable(t, path)
}

// crDedupeUnionRows rewrites path when update mode is on and two rows share a
// rule id. Rows for the same rule are merged in place (first occurrence wins
// the position); rows for other rules are untouched.
func crDedupeUnionRows(t *testing.T, path string) bool {
	t.Helper()
	if os.Getenv(crCoverageUpdateEnv) != "1" {
		return false
	}
	lines, rows, err := crScanTable(path)
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]crRow{}
	var order []string
	for _, r := range rows {
		if _, ok := groups[r.rule]; !ok {
			order = append(order, r.rule)
		}
		groups[r.rule] = append(groups[r.rule], r)
	}
	replace := map[int]string{}
	remove := map[int]bool{}
	for _, rule := range order {
		g := groups[rule]
		if len(g) < 2 {
			continue
		}
		replace[g[0].line-1] = crFormatRow(crMergeRows(g))
		for _, r := range g[1:] {
			remove[r.line-1] = true
		}
	}
	if len(replace) == 0 {
		return false
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if remove[i] {
			continue
		}
		if merged, ok := replace[i]; ok {
			out = append(out, merged)
			continue
		}
		out = append(out, line)
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("collapsed duplicate rule rows in %s", path)
	return true
}

// crScanTable returns the file's lines and the rows parsed from its table. It
// skips lines readCRTable would reject, so update mode never hides a malformed
// row: the normal validation still reports it afterwards.
func crScanTable(path string) ([]string, []crRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(data), "\n")
	var rows []crRow
	inTable := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == crCoverageHeader:
			inTable = true
		case !inTable:
		case !strings.HasPrefix(line, "|"):
			inTable = false
		case strings.HasPrefix(line, "|---") || strings.HasPrefix(line, "| ---"):
		default:
			cells := strings.Split(strings.Trim(line, "|"), "|")
			if len(cells) != 5 {
				continue
			}
			for j := range cells {
				cells[j] = strings.TrimSpace(cells[j])
			}
			rows = append(rows, crRow{
				file: filepath.Base(path), line: i + 1,
				rule: cells[0], status: cells[1],
				code: crFields(cells[2]), tests: crFields(cells[3]),
				notes: cells[4],
			})
		}
	}
	return lines, rows, nil
}

// crMergeRows folds duplicate rows for one rule into one row. Code and test
// references are the order-preserving union; differing notes are both kept.
func crMergeRows(rows []crRow) crRow {
	m := crRow{file: rows[0].file, rule: rows[0].rule, line: rows[0].line}
	seenCode := map[string]bool{}
	seenTest := map[string]bool{}
	seenNote := map[string]bool{}
	var notes []string
	for _, r := range rows {
		for _, c := range r.code {
			if !seenCode[c] {
				seenCode[c] = true
				m.code = append(m.code, c)
			}
		}
		for _, tr := range r.tests {
			if !seenTest[tr] {
				seenTest[tr] = true
				m.tests = append(m.tests, tr)
			}
		}
		n := strings.TrimSpace(r.notes)
		if n == "" || n == "-" || seenNote[n] {
			continue
		}
		seenNote[n] = true
		notes = append(notes, n)
	}
	m.notes = strings.Join(notes, "; ")
	m.status = crMergeStatus(rows)
	return m
}

// crMergeStatus keeps the agreed status. On a conflict a partial or deviates
// reading beats follows (never overclaim coverage); otherwise the first
// non-follows status wins. The rule id is logged so a train leaves a trail.
func crMergeStatus(rows []crRow) string {
	status := rows[0].status
	agree := true
	for _, r := range rows[1:] {
		if r.status != status {
			agree = false
			break
		}
	}
	if agree {
		return status
	}
	var statuses []string
	hasPartial, hasDeviates := false, false
	for _, r := range rows {
		statuses = append(statuses, r.status)
		switch r.status {
		case "partial":
			hasPartial = true
		case "deviates":
			hasDeviates = true
		}
	}
	merged := status
	switch {
	case hasDeviates:
		merged = "deviates"
	case hasPartial:
		merged = "partial"
	default:
		for _, s := range statuses {
			if s != "follows" {
				merged = s
				break
			}
		}
	}
	fmt.Printf("crcoverage: rule %s: conflicting statuses [%s] -> %s\n",
		rows[0].rule, strings.Join(statuses, ", "), merged)
	return merged
}

// crFormatRow writes a row in the table's canonical cell spacing.
func crFormatRow(r crRow) string {
	return "| " + r.rule + " | " + r.status + " | " + crCell(r.code) + " | " +
		crCell(r.tests) + " | " + crNotesCell(r.notes) + " |"
}

func crCell(fields []string) string {
	if len(fields) == 0 {
		return "-"
	}
	return strings.Join(fields, " ")
}

func crNotesCell(notes string) string {
	if notes == "" {
		return "-"
	}
	return notes
}

func TestCRCoverageUnionDedupe(t *testing.T) {
	const table = "# CR 600s: test\n\n| rule | status | code | tests | notes |\n|---|---|---|---|---|\n"

	t.Run("same status", func(t *testing.T) {
		t.Setenv(crCoverageUpdateEnv, "1")
		path := filepath.Join(t.TempDir(), "600-test.md")
		body := table +
			"| 601.2f | follows | `dirac/a.go#A` `dirac/b.go#B` | `TestA` | first note |\n" +
			"| 601.2f | follows | `dirac/b.go#B` `dirac/c.go#C` | `TestB` | second note |\n" +
			"| 601.2g | follows | `dirac/d.go#D` | - | tail note |\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rows := crReadChapter(t, path)
		if len(rows) != 2 {
			t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
		}
		r := rows[0]
		if r.rule != "601.2f" || r.status != "follows" {
			t.Fatalf("merged row is %q/%q, want 601.2f/follows", r.rule, r.status)
		}
		if got, want := strings.Join(r.code, " "), "`dirac/a.go#A` `dirac/b.go#B` `dirac/c.go#C`"; got != want {
			t.Errorf("code = %s, want %s", got, want)
		}
		if got, want := strings.Join(r.tests, " "), "`TestA` `TestB`"; got != want {
			t.Errorf("tests = %s, want %s", got, want)
		}
		if want := "first note; second note"; r.notes != want {
			t.Errorf("notes = %q, want %q", r.notes, want)
		}
		if rows[1].rule != "601.2g" {
			t.Errorf("second row = %q, want the untouched 601.2g", rows[1].rule)
		}
	})

	t.Run("conflicting status", func(t *testing.T) {
		t.Setenv(crCoverageUpdateEnv, "1")
		path := filepath.Join(t.TempDir(), "600-test.md")
		body := table +
			"| 601.2f | follows | `dirac/a.go#A` | - | note |\n" +
			"| 601.2f | partial | `dirac/b.go#B` | - | note |\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rows := crReadChapter(t, path)
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
		}
		if rows[0].status != "partial" {
			t.Errorf("status = %q, want the conservative partial", rows[0].status)
		}
	})

	t.Run("not update mode", func(t *testing.T) {
		t.Setenv(crCoverageUpdateEnv, "0")
		path := filepath.Join(t.TempDir(), "600-test.md")
		body := table +
			"| 601.2f | follows | `dirac/a.go#A` | - | note |\n" +
			"| 601.2f | follows | `dirac/b.go#B` | - | note |\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if rows := crReadChapter(t, path); len(rows) != 2 {
			t.Fatalf("got %d rows, want the unreconciled 2", len(rows))
		}
	})
}
