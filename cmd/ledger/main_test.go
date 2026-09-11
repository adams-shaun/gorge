package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A lane's meaning depends entirely on counting LEAVES. A parent test that
// fails only because one subtest failed is the same defect counted twice, and
// a lane whose total drifts with the shape of the test tree stops being a
// measurement. Both shapes below appear in the real lane.
func TestLaneEntriesCountsLeavesOnly(t *testing.T) {
	lane := `=== RUN   TestCR601Targets
=== RUN   TestCR601Targets/Bolt
--- FAIL: TestCR601Targets (0.10s)
    --- FAIL: TestCR601Targets/Bolt (0.05s)
    --- PASS: TestCR601Targets/Shock (0.05s)
--- PASS: TestCR602Alone (0.01s)
--- SKIP: TestCR603Skipped (0.00s)
`
	got := laneEntries(lane, map[string]doc{
		"TestCR601Targets": {title: "targets precede payment.", where: "rules/a_test.go:10"},
	})
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3 (two subtest leaves + one standalone); "+
			"the parent TestCR601Targets must not be counted, and a SKIP is not a leaf: %+v", len(got), got)
	}
	byID := map[string]Entry{}
	for _, e := range got {
		byID[e.ID] = e
	}
	if _, ok := byID["TestCR601Targets"]; ok {
		t.Fatal("the parent test was counted as a leaf, double-counting its subtest's defect")
	}
	if e := byID["TestCR601Targets/Bolt"]; e.Status != "open" {
		t.Fatalf("failing leaf status = %q, want open", e.Status)
	}
	// A subtest carries no doc comment of its own; it must inherit the parent
	// func's location so the row still points somewhere.
	if e := byID["TestCR601Targets/Shock"]; e.Status != "closed" || e.Where != "rules/a_test.go:10" {
		t.Fatalf("passing leaf = %+v, want closed at the parent func's location", e)
	}
	if _, ok := byID["TestCR603Skipped"]; ok {
		t.Fatal("a SKIP was recorded; a skipped test measured nothing and is not a ledger row")
	}
}

// The approximations table's description cells carry Forge script fragments,
// and those fragments have their OWN "|" separators. Splitting on "|" and
// taking cells[1] as Where therefore put a script fragment in the location
// column on four real rows. Where and Removed by are the LAST two cells.
func TestApproximationsSplitsOnTheLastTwoCells(t *testing.T) {
	md := "## Known approximations\n\n" +
		"| Stand-in | Where | Removed by |\n|---|---|---|\n" +
		"| plain row | `effects/misc.go:480` | M4 |\n" +
		"| a row whose text has `DB$ Discard | Mode$ TgtChoose` inside it | `effects/cardflow.go` | M5 |\n" +
		"\n## Next section\n" +
		"| not a row | in another table | ignored |\n"
	got := approximations(md)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 — the header, the separator and the next section's table must all be excluded: %+v", len(got), got)
	}
	if got[0].Where != "`effects/misc.go:480`" || got[0].Disposition != "approved — removed by M4" {
		t.Fatalf("plain row = %+v", got[0])
	}
	if got[1].Where != "`effects/cardflow.go`" {
		t.Fatalf("Where = %q, want the LAST-but-one cell; a script fragment in the description "+
			"must not be mistaken for the location", got[1].Where)
	}
	if got[1].Disposition != "approved — removed by M5" {
		t.Fatalf("Disposition = %q", got[1].Disposition)
	}
	if want := "a row whose text has `DB$ Discard | Mode$ TgtChoose` inside it"; got[1].Title != want {
		t.Fatalf("Title = %q, want the description rejoined with its own separators intact", got[1].Title)
	}
}

// A row with fewer than three cells names no location and no milestone, which
// is the one shape the table's header forbids. Dropping it silently is how a
// broken row survives; it is surfaced as open instead.
func TestApproximationsSurfacesAMalformedRow(t *testing.T) {
	md := "## Known approximations\n\n| Stand-in | Where | Removed by |\n|---|---|---|\n" +
		"| a row that lost its columns\n"
	got := approximations(md)
	if len(got) != 1 || got[0].Status != "open" {
		t.Fatalf("got %+v, want one open row flagging the malformed entry", got)
	}
}

func TestFirstSentenceStripsTheGoDocNamePrefix(t *testing.T) {
	if got := firstSentence("TestFoo checks a thing. And more.\n"); got != "checks a thing." {
		t.Fatalf("got %q", got)
	}
	if got := firstSentence(""); got != "" {
		t.Fatalf("empty doc must stay empty, got %q", got)
	}
}

// writeIssue drops one issue file into a temp issues dir, mirroring the
// orchestrator's frontmatter shape. Fixtures are inline; nothing is read from
// the live .ds4 (a fresh worktree has none).
func writeIssue(t *testing.T, dir, name, frontmatter string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
}

const issueFixture = "---\nid: %s\ntitle: %s\nstatus: %s\ncommits: %s\n---\n\nBody prose.\n"

// The statuses observed in the live .ds4/issues at the time of the change.
// merged is the ONLY closed value — a human_needed defect is still an unfixed
// defect (it needs a human, which is what its disposition says) — and every
// active pipeline status opens with a disposition naming where it stands.
func TestIssueEntriesMapsTheObservedStatuses(t *testing.T) {
	dir := t.TempDir()
	writeIssue(t, dir, "a.md", "---\nid: fb-one\ntitle: a merged defect\nstatus: merged\ncommits: 588a935\n---\n")
	writeIssue(t, dir, "b.md", "---\nid: fb-two\ntitle: a human-needed defect\nstatus: human_needed\n---\n")
	writeIssue(t, dir, "c.md", "---\nid: fb-three\ntitle: a fresh report\nstatus: new\n---\n")
	writeIssue(t, dir, "d.md", "---\nid: fb-four\ntitle: a dispatched report\nstatus: dispatched\n---\n")
	got, err := issueEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d entries, want 4: %+v", len(got), got)
	}
	byID := map[string]Entry{}
	for _, e := range got {
		byID[e.ID] = e
	}
	if e := byID["issue-fb-one"]; e.Kind != "report" || e.Status != "closed" || e.Disposition != "merged — 588a935" {
		t.Fatalf("merged = %+v", e)
	}
	if e := byID["issue-fb-two"]; e.Status != "open" || e.Disposition != "needs human" {
		t.Fatalf("human_needed = %+v", e)
	}
	if e := byID["issue-fb-three"]; e.Status != "open" || e.Disposition != "reported — awaiting triage" {
		t.Fatalf("new = %+v", e)
	}
	if e := byID["issue-fb-four"]; e.Status != "open" || e.Disposition != "dispatched — seat active" {
		t.Fatalf("dispatched = %+v", e)
	}
}

// A merged issue with no commits field must not render an empty "merged — ".
func TestIssueEntriesMergedWithoutCommitsFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeIssue(t, dir, "a.md", "---\nid: fb-one\ntitle: t\nstatus: merged\n---\n")
	got, err := issueEntries(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if got[0].Disposition != "merged" {
		t.Fatalf("Disposition = %q, want the bare fallback", got[0].Disposition)
	}
}

// A status the mapping does not list must still OPEN — never silently close —
// and its disposition must name the value honestly so the row reads as an
// unknown, not a resolution.
func TestIssueEntriesOpensAnUnlistedStatusHonestly(t *testing.T) {
	dir := t.TempDir()
	writeIssue(t, dir, "a.md", "---\nid: fb-x\ntitle: t\nstatus: quarantined\n---\n")
	got, err := issueEntries(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if got[0].Status != "open" || got[0].Disposition != "open — status quarantined" {
		t.Fatalf("unlisted status = %+v", got[0])
	}
}

// inbox/ is the un-triaged drop zone (orchestrator/config.py INBOX_DIR); a
// ticket there becomes a top-level file at triage, so reading it would
// double-count one defect. Subdirectories are skipped entirely, as is a file
// that is not .md and one with no parseable frontmatter id — a malformed file
// in a human drop zone must not kill `make ledger`.
func TestIssueEntriesSkipsTheInboxSubdirAndMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	writeIssue(t, dir, "a.md", "---\nid: fb-real\ntitle: real\nstatus: new\n---\n")
	if err := os.MkdirAll(filepath.Join(dir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssue(t, dir, filepath.Join("inbox", "dropped.md"), "---\nid: fb-inbox\ntitle: dropped\nstatus: new\n---\n")
	writeIssue(t, dir, "notes.txt", "---\nid: fb-txt\n---\n")
	writeIssue(t, dir, "broken.md", "no fences at all, just prose.\n")
	writeIssue(t, dir, "fenced-empty.md", "---\ntitle: no id key\nstatus: new\n---\n")
	writeIssue(t, dir, "unterminated.md", "---\nid: fb-unterm\ntitle: t\nstatus: new\nbody keeps going without a closing fence\n")
	got, err := issueEntries(dir)
	if err != nil {
		t.Fatalf("a malformed file must not be fatal: %v", err)
	}
	if len(got) != 1 || got[0].ID != "issue-fb-real" {
		t.Fatalf("got %+v, want exactly the one well-formed top-level issue", got)
	}
	// Title arrives whitespace-normalised from pre-truncated player text.
	writeIssue(t, dir, "b.md", "---\nid: fb-ws\ntitle: spaced   out\tttitle\nstatus: new\n---\n")
	got, err = issueEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.ID == "issue-fb-ws" && e.Title != "spaced out ttitle" {
			t.Fatalf("Title = %q, want whitespace-normalised", e.Title)
		}
	}
}

// A missing dir is an empty source, not an error — a fresh worktree has no
// .ds4/issues at all, and `make ledger` must not die on it.
func TestIssueEntriesTreatsAMissingDirAsAnEmptySource(t *testing.T) {
	got, err := issueEntries(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir = %v, want nil error", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want no entries", got)
	}
}
