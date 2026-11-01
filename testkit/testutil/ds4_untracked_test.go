package testutil_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestNothingUnderDS4IsTracked: .ds4/ is a gitignored scratch directory that
// tools overwrite in place. A tracked file there breaks every rebase on
// "You have unstaged changes", and a branch that commits one carries its churn.
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
		t.Fatalf("%d path(s) under .ds4/ are tracked; .ds4 is scratch and must stay untracked (git rm --cached): %v", len(tracked), tracked)
	}
}
