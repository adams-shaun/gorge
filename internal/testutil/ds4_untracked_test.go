package testutil_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestNothingUnderDS4IsTracked: .ds4/ is the pipeline's per-worktree scratch
// (briefs, seat reports, ledger) and is gitignored, but 31 historical seat
// reports had been force-committed there. agentctl writes each round's report
// to .ds4/report-<tag>.md, so every seat overwrote a TRACKED file: approved
// branches then failed their pre-merge rebase on "You have unstaged changes"
// (agent-...-09ee8c17, fb-...-232ca3dc, 2026-09-24), and branches that
// committed the report carried thousands of lines of report churn. Nothing
// under .ds4/ may be tracked again.
func TestNothingUnderDS4IsTracked(t *testing.T) {
	top := exec.Command("git", "rev-parse", "--show-toplevel")
	top.Env = cards.GitEnv()
	root, err := top.Output()
	if err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	ls := exec.Command("git", "-C", strings.TrimSpace(string(root)), "ls-files", "--", ".ds4")
	ls.Env = cards.GitEnv()
	out, err := ls.Output()
	if err != nil {
		t.Fatalf("git ls-files .ds4: %v", err)
	}
	if tracked := strings.Fields(string(out)); len(tracked) > 0 {
		t.Fatalf("%d path(s) under .ds4/ are tracked; .ds4 is pipeline scratch and must stay untracked (git rm --cached): %v", len(tracked), tracked)
	}
}
