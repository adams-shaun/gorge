package archtest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A rule citation in engine Go source ("CR 603.2a", or a bare section "CR 603")
// must name an id of the pinned Comprehensive Rules text, as committed in
// engine/rules/crcoverage/cr-index.tsv. Ids that name no rule today sit in the
// shrink-only baseline testdata/cr-citations-unknown.tsv with their citation
// count: a count may only fall, and a line whose id is no longer cited (or is
// now a rule) must be deleted.

var crCitationRE = regexp.MustCompile(`CR (\d{3}(?:\.\d+[a-z]?)?)`)

const (
	crIndexPath         = "../../engine/rules/crcoverage/cr-index.tsv"
	crUnknownBaseline   = "testdata/cr-citations-unknown.tsv"
	crCitationScopePref = "engine/"
)

// crCitation is one rule citation in a tracked engine Go file.
type crCitation struct {
	path string // repo-relative, slash-separated
	line int    // 1-based
	id   string // the cited rule id, e.g. "603.2a"
	text string // the whole source line
}

// crCitations scans every tracked .go file under engine/ (test files included)
// and returns its rule citations in path, line, then column order.
func crCitations(t *testing.T) []crCitation {
	t.Helper()
	var out []crCitation
	walkPublicFiles(t, func(p string, blob []byte) {
		if !strings.HasPrefix(p, crCitationScopePref) || !strings.HasSuffix(p, ".go") {
			return
		}
		for i, line := range strings.Split(string(blob), "\n") {
			for _, m := range crCitationRE.FindAllStringSubmatch(line, -1) {
				out = append(out, crCitation{path: p, line: i + 1, id: m[1], text: line})
			}
		}
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].line < out[j].line
	})
	return out
}

// readCRIndexIDs returns column 1 of the committed rules index.
func readCRIndexIDs(t *testing.T) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for i, row := range readTSV(t, filepath.FromSlash(crIndexPath)) {
		if i == 0 {
			continue // header
		}
		ids[row[0]] = true
	}
	if len(ids) == 0 {
		t.Fatalf("%s: no rule ids", crIndexPath)
	}
	return ids
}

// readCRUnknownBaseline returns the baseline's id -> citation count.
func readCRUnknownBaseline(t *testing.T) map[string]int {
	t.Helper()
	base := map[string]int{}
	rows := readTSV(t, filepath.FromSlash(crUnknownBaseline))
	if len(rows) == 0 || strings.Join(rows[0], "\t") != "id\tcites" {
		t.Fatalf("%s: header must be \"id\\tcites\"", crUnknownBaseline)
	}
	var order []string
	for _, row := range rows[1:] {
		if len(row) != 2 {
			t.Fatalf("%s: row %q: want 2 columns", crUnknownBaseline, strings.Join(row, "\t"))
		}
		n, err := strconv.Atoi(row[1])
		if err != nil || n <= 0 {
			t.Fatalf("%s: row %q: cites must be a positive integer", crUnknownBaseline, strings.Join(row, "\t"))
		}
		if _, dup := base[row[0]]; dup {
			t.Fatalf("%s: duplicate id %s", crUnknownBaseline, row[0])
		}
		base[row[0]] = n
		order = append(order, row[0])
	}
	if !sort.SliceIsSorted(order, func(i, j int) bool { return crRuleLess(order[i], order[j]) }) {
		t.Fatalf("%s: rows must be sorted by id in rule order", crUnknownBaseline)
	}
	return base
}

// readTSV reads a tab-separated file, skipping blank lines.
func readTSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var rows [][]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			rows = append(rows, strings.Split(line, "\t"))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return rows
}

// crRuleKey splits a rule id into section, rule number and letter.
func crRuleKey(id string) (section, rule int, letter string) {
	sec, rest, _ := strings.Cut(id, ".")
	section, _ = strconv.Atoi(sec)
	digits := strings.TrimRight(rest, "abcdefghijklmnopqrstuvwxyz")
	rule, _ = strconv.Atoi(digits)
	return section, rule, rest[len(digits):]
}

// crRuleLess orders rule ids as the rules text does: section, then rule
// number, then letter ("603" < "603.1" < "603.1a" < "603.2" < "603.10").
func crRuleLess(a, b string) bool {
	as, ar, al := crRuleKey(a)
	bs, br, bl := crRuleKey(b)
	if as != bs {
		return as < bs
	}
	if ar != br {
		return ar < br
	}
	if al != bl {
		return al < bl
	}
	return a < b
}

// crCitationProblems compares the cited ids (id -> citation count) with the
// rules index and the unknown-id baseline. Messages prefixed "log: " are
// progress to report, not failures. The result is sorted.
func crCitationProblems(found map[string]int, index map[string]bool, baseline map[string]int) []string {
	var out []string
	for id, n := range found {
		if index[id] {
			continue
		}
		m, inBase := baseline[id]
		switch {
		case !inBase:
			out = append(out, fmt.Sprintf("%s: cited %d times, not a rule of the pinned text", id, n))
		case n > m:
			out = append(out, fmt.Sprintf("%s: %d citations, baseline %d: new citations of an id that does not exist", id, n, m))
		case n < m:
			out = append(out, fmt.Sprintf("log: %s: baseline %d, found %d: lower the count", id, m, n))
		}
	}
	for id := range baseline {
		if found[id] == 0 || index[id] {
			out = append(out, fmt.Sprintf("%s: no longer cited or now in the index: delete the baseline line", id))
		}
	}
	sort.Strings(out)
	return out
}

func TestCRCitationsNameRealRules(t *testing.T) {
	t.Parallel()
	cites := crCitations(t)
	found := map[string]int{}
	for _, c := range cites {
		found[c.id]++
	}
	t.Logf("%d citations of %d distinct ids under %s", len(cites), len(found), crCitationScopePref)
	index := readCRIndexIDs(t)
	baseline := readCRUnknownBaseline(t)
	for _, msg := range crCitationProblems(found, index, baseline) {
		if rest, ok := strings.CutPrefix(msg, "log: "); ok {
			t.Logf("%s", rest)
			continue
		}
		id, _, _ := strings.Cut(msg, ":")
		var where []string
		for _, c := range cites {
			if c.id == id && len(where) < 5 {
				where = append(where, fmt.Sprintf("%s:%d", c.path, c.line))
			}
		}
		if len(where) > 0 {
			msg += " (" + strings.Join(where, ", ") + ")"
		}
		t.Errorf("%s (%s)", msg, crUnknownBaseline)
	}
}

func TestCRCitationProblems(t *testing.T) {
	t.Parallel()
	index := map[string]bool{"603": true, "603.2": true, "603.2a": true}
	cases := []struct {
		name     string
		found    map[string]int
		baseline map[string]int
		want     []string
	}{
		{
			name:     "clean",
			found:    map[string]int{"603": 1, "603.2a": 4, "707.10h": 3},
			baseline: map[string]int{"707.10h": 3},
			want:     nil,
		},
		{
			name:  "unknown id not in baseline",
			found: map[string]int{"603.2a": 1, "999.9": 2},
			want:  []string{"999.9: cited 2 times, not a rule of the pinned text"},
		},
		{
			name:     "count above baseline",
			found:    map[string]int{"707.10h": 5},
			baseline: map[string]int{"707.10h": 3},
			want:     []string{"707.10h: 5 citations, baseline 3: new citations of an id that does not exist"},
		},
		{
			name:     "count below baseline is a log",
			found:    map[string]int{"707.10h": 2},
			baseline: map[string]int{"707.10h": 3},
			want:     []string{"log: 707.10h: baseline 3, found 2: lower the count"},
		},
		{
			name:     "baseline id no longer cited",
			found:    map[string]int{"603": 1},
			baseline: map[string]int{"707.10h": 3},
			want:     []string{"707.10h: no longer cited or now in the index: delete the baseline line"},
		},
		{
			name:     "baseline id now in the index",
			found:    map[string]int{"603.2": 3},
			baseline: map[string]int{"603.2": 3},
			want:     []string{"603.2: no longer cited or now in the index: delete the baseline line"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := crCitationProblems(tc.found, index, tc.baseline)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
			}
		})
	}
	if !crRuleLess("603", "603.1") || !crRuleLess("603.1a", "603.2") || !crRuleLess("603.2", "603.10") || !crRuleLess("603.10", "603.10a") {
		t.Errorf("crRuleLess: rule order broken")
	}
}
