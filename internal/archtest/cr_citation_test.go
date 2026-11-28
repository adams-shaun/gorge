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
	"sync"
	"testing"
	"unicode"
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

// A citation of a keyword action (701.N) or keyword ability (702.N) whose line
// names a different keyword than the cited rule's title was written against
// another numbering of the rules text (enlist cited as 702.160, Prototype).
// Lines flagged today sit in the shrink-only per-file baseline
// testdata/cr-citations-keyword-mismatch.tsv; scripts/cr_cite_renumber.py
// rewrites them a slice at a time.

const crKeywordBaseline = "testdata/cr-citations-keyword-mismatch.tsv"

// crKeywordMinLetters is the shortest title that can flag a line: shorter
// titles ("Cast", "Vote") read as ordinary words too often.
const crKeywordMinLetters = 5

// crKeywordRuleRE matches a rule id of section 701 or 702 and splits off its
// rule number; the letter, if any, is dropped.
var crKeywordRuleRE = regexp.MustCompile(`^(70[12]\.(\d+))[a-z]?$`)

// crKeywordRule returns the keyword rule a cited id belongs to ("702.160a" ->
// "702.160"), or false for any id outside 701.2+ and 702.2+.
func crKeywordRule(id string) (string, bool) {
	m := crKeywordRuleRE.FindStringSubmatch(id)
	if m == nil {
		return "", false
	}
	if n, _ := strconv.Atoi(m[2]); n < 2 {
		return "", false
	}
	return m[1], true
}

// readCRTitles returns rule id -> title for every index row of kind "rule"
// with a non-empty title.
func readCRTitles(t *testing.T) map[string]string {
	t.Helper()
	titles := map[string]string{}
	for i, row := range readTSV(t, filepath.FromSlash(crIndexPath)) {
		if i == 0 || len(row) < 3 || row[1] != "rule" || row[2] == "" {
			continue
		}
		titles[row[0]] = row[2]
	}
	if len(titles) == 0 {
		t.Fatalf("%s: no titled rules", crIndexPath)
	}
	return titles
}

var crTitleREs sync.Map // lower-cased title -> *regexp.Regexp

// crContainsWord reports whether lower (a lower-cased line) holds title as a
// whole word, case-insensitively.
func crContainsWord(lower, title string) bool {
	t := strings.ToLower(title)
	if !strings.Contains(lower, t) {
		return false
	}
	re, ok := crTitleREs.Load(t)
	if !ok {
		re, _ = crTitleREs.LoadOrStore(t, regexp.MustCompile(`\b`+regexp.QuoteMeta(t)+`\b`))
	}
	return re.(*regexp.Regexp).MatchString(lower)
}

func crLetterCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// crKeywordMismatch reports whether line cites citedRule (a 701 or 702 rule
// id) without containing its title while naming, as a whole word, the title
// (five or more letters) of a different 701 or 702 rule. named is the first
// such rule in rule order.
func crKeywordMismatch(line string, citedRule string, titles map[string]string) (named string, bad bool) {
	own, ok := titles[citedRule]
	if !ok {
		return "", false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, strings.ToLower(own)) {
		return "", false
	}
	var ids []string
	for id := range titles {
		if _, kw := crKeywordRule(id); kw && id != citedRule {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return crRuleLess(ids[i], ids[j]) })
	for _, id := range ids {
		title := titles[id]
		if crLetterCount(title) >= crKeywordMinLetters && crContainsWord(lower, title) {
			return id, true
		}
	}
	return "", false
}

// crKeywordFlag is one flagged line.
type crKeywordFlag struct {
	path, cited, named string
	line               int
}

// crKeywordFlags returns the flagged lines of the engine scan, one per line
// (the first flagged citation on it), in path then line order.
func crKeywordFlags(t *testing.T) []crKeywordFlag {
	t.Helper()
	titles := readCRTitles(t)
	var out []crKeywordFlag
	seen := map[string]bool{}
	for _, c := range crCitations(t) {
		rule, ok := crKeywordRule(c.id)
		if !ok {
			continue
		}
		key := c.path + ":" + strconv.Itoa(c.line)
		if seen[key] {
			continue
		}
		if named, bad := crKeywordMismatch(c.text, rule, titles); bad {
			seen[key] = true
			out = append(out, crKeywordFlag{path: c.path, line: c.line, cited: c.id, named: named})
		}
	}
	return out
}

// readCRKeywordBaseline returns the baseline's path -> flagged line count.
func readCRKeywordBaseline(t *testing.T) map[string]int {
	t.Helper()
	base := map[string]int{}
	rows := readTSV(t, filepath.FromSlash(crKeywordBaseline))
	if len(rows) == 0 || strings.Join(rows[0], "\t") != "path\tlines" {
		t.Fatalf("%s: header must be \"path\\tlines\"", crKeywordBaseline)
	}
	var order []string
	for _, row := range rows[1:] {
		if len(row) != 2 {
			t.Fatalf("%s: row %q: want 2 columns", crKeywordBaseline, strings.Join(row, "\t"))
		}
		n, err := strconv.Atoi(row[1])
		if err != nil || n <= 0 {
			t.Fatalf("%s: row %q: lines must be a positive integer", crKeywordBaseline, strings.Join(row, "\t"))
		}
		if _, dup := base[row[0]]; dup {
			t.Fatalf("%s: duplicate path %s", crKeywordBaseline, row[0])
		}
		base[row[0]] = n
		order = append(order, row[0])
	}
	if !sort.StringsAreSorted(order) {
		t.Fatalf("%s: rows must be sorted by path", crKeywordBaseline)
	}
	return base
}

// crKeywordProblems compares flagged line counts per file with the baseline.
// Messages prefixed "log: " are progress, not failures. The result is sorted.
func crKeywordProblems(found, baseline map[string]int) []string {
	var out []string
	for p, n := range found {
		m, inBase := baseline[p]
		switch {
		case !inBase:
			out = append(out, fmt.Sprintf("%s: %d lines cite one keyword's rule beside another keyword's name, file not in the baseline", p, n))
		case n > m:
			out = append(out, fmt.Sprintf("%s: %d mismatched keyword citations, baseline %d: new mismatches", p, n, m))
		case n < m:
			out = append(out, fmt.Sprintf("log: %s: baseline %d, found %d: lower the count", p, m, n))
		}
	}
	for p := range baseline {
		if found[p] == 0 {
			out = append(out, fmt.Sprintf("%s: no mismatched keyword citations left: delete the baseline line", p))
		}
	}
	sort.Strings(out)
	return out
}

func TestCRKeywordCitationsNameTheirKeyword(t *testing.T) {
	t.Parallel()
	flags := crKeywordFlags(t)
	found := map[string]int{}
	for _, f := range flags {
		found[f.path]++
	}
	t.Logf("%d flagged lines in %d files under %s", len(flags), len(found), crCitationScopePref)
	baseline := readCRKeywordBaseline(t)
	for _, msg := range crKeywordProblems(found, baseline) {
		if rest, ok := strings.CutPrefix(msg, "log: "); ok {
			t.Logf("%s", rest)
			continue
		}
		path, _, _ := strings.Cut(msg, ":")
		var where []string
		for _, f := range flags {
			if f.path == path && len(where) < 5 {
				where = append(where, fmt.Sprintf("%s:%d cites %s, names %s", f.path, f.line, f.cited, f.named))
			}
		}
		if len(where) > 0 {
			msg += " (" + strings.Join(where, "; ") + ")"
		}
		t.Errorf("%s (%s)", msg, crKeywordBaseline)
	}
}

func TestCRKeywordMismatch(t *testing.T) {
	t.Parallel()
	titles := map[string]string{
		"702.2":   "Deathtouch",
		"702.154": "Enlist",
		"702.160": "Prototype",
		"701.38":  "Vote",
		"701.2":   "Activate",
		"603.2":   "Triggered Ability", // not a keyword rule: never named
	}
	cases := []struct {
		line, cited string
		named       string
		bad         bool
	}{
		{"// CR 702.160: enlist taps a creature", "702.160", "702.154", true},
		{"// CR 702.160: prototype", "702.160", "", false},
		{"// CR 702.160: Prototype and enlist both", "702.160", "", false},
		{"// CR 702.154: players vote", "702.154", "", false},          // Vote has four letters
		{"// CR 702.154: enlisted creature", "702.154", "", false},     // own title as a substring
		{"// CR 702.160: enlisting a creature", "702.160", "", false},  // other title not a whole word
		{"// CR 702.2: enlist, then activate", "702.2", "701.2", true}, // first in rule order
		{"// CR 702.160: a triggered ability", "702.160", "", false},
		{"// CR 702.999: enlist", "702.999", "", false}, // no title for the cited rule
	}
	for _, tc := range cases {
		named, bad := crKeywordMismatch(tc.line, tc.cited, titles)
		if named != tc.named || bad != tc.bad {
			t.Errorf("crKeywordMismatch(%q, %s) = %q, %v; want %q, %v", tc.line, tc.cited, named, bad, tc.named, tc.bad)
		}
	}
	for id, want := range map[string]string{"702.160a": "702.160", "701.2": "701.2", "701.1": "", "702.1b": "", "702": "", "603.2": ""} {
		if got, _ := crKeywordRule(id); got != want {
			t.Errorf("crKeywordRule(%s) = %q, want %q", id, got, want)
		}
	}
	got := crKeywordProblems(map[string]int{"a.go": 2, "b.go": 3, "c.go": 1, "e.go": 1}, map[string]int{"a.go": 2, "b.go": 2, "c.go": 4, "d.go": 1})
	want := []string{
		"b.go: 3 mismatched keyword citations, baseline 2: new mismatches",
		"d.go: no mismatched keyword citations left: delete the baseline line",
		"e.go: 1 lines cite one keyword's rule beside another keyword's name, file not in the baseline",
		"log: c.go: baseline 4, found 1: lower the count",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("crKeywordProblems:\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}
