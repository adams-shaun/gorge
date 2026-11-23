package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentsDocsCarryDeletionAndTestSpendRules pins the deletion rule and the
// test-spend rows in the pages an agent reads. AGENTS.md is the entry point; the
// detail lives in docs/agents/, so both links must stay and the text must stay
// in the named sections.
func TestAgentsDocsCarryDeletionAndTestSpendRules(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	agents := read("AGENTS.md")
	invariants := read("docs/agents/invariants.md")
	doNot := read("docs/agents/do-not.md")

	// Precondition: each rule is in the section named by the brief, so a match
	// cannot come from an unrelated part of the page.
	for _, doc := range []struct {
		name    string
		text    string
		heading string
	}{
		{"AGENTS.md", agents, "## Agent guide"},
		{"docs/agents/invariants.md", invariants, "### Workflow"},
		{"docs/agents/do-not.md", doNot, "## Tests, ratchets and docs"},
	} {
		if !strings.Contains(doc.text, doc.heading) {
			t.Fatalf("%s no longer has %q; the rules are pinned in the wrong place", doc.name, doc.heading)
		}
	}

	workflow := invariants[strings.Index(invariants, "### Workflow"):]
	for _, clause := range []string{
		"It is a duplicate by construction.",
		"It has become data.",
		"Its subject is gone.",
		"Never delete a test because it has not failed, or because it is slow.",
	} {
		if !strings.Contains(workflow, clause) {
			t.Errorf("docs/agents/invariants.md Workflow is missing the deletion rule clause %q", clause)
		}
	}

	rules := doNot[strings.Index(doNot, "## Tests, ratchets and docs"):]
	for _, row := range []string{
		"Don't write a Go test function per recording.",
		"Don't delete a test because it never failed.",
		"Don't run raw `go test` instead of the make targets.",
		"Don't add a new test over 60 s at 4 cores.",
	} {
		if !strings.Contains(rules, row) {
			t.Errorf("docs/agents/do-not.md is missing the test-spend row %q", row)
		}
	}

	for _, link := range []string{"docs/agents/invariants.md", "docs/agents/do-not.md"} {
		if !strings.Contains(agents, link) {
			t.Errorf("AGENTS.md does not link %s, so a reader never reaches the rule", link)
		}
	}
}
