package scripts

// ci-fast.sh runs TestNothingUnderDS4IsTracked on every pull request, whatever the diff selects (11-23: a seat's
// force-added .ds4 report passed the PR's checks and reddened two merge batches).

import (
	"os"
	"strings"
	"testing"
)

func TestCIFastAlwaysChecksDS4Untracked(t *testing.T) {
	b, err := os.ReadFile("ci-fast.sh")
	if err != nil {
		t.Fatal(err)
	}
	const want = "go test -timeout 2m -short -run '^TestNothingUnderDS4IsTracked$' ./testkit/testutil || rc=1"
	s := string(b)
	i := strings.Index(s, want)
	if i < 0 {
		t.Fatalf("ci-fast.sh no longer runs TestNothingUnderDS4IsTracked unconditionally; want the line\n%s", want)
	}
	// It must sit at top level, after the per-diff loop, not inside a branch that a diff can skip.
	if j := strings.LastIndex(s, "done <<<\"$list\""); j < 0 || j > i {
		t.Fatalf("the .ds4 check must follow the per-diff test loop at top level")
	}
	if i == 0 || s[i-1] != '\n' {
		t.Fatalf("the .ds4 check must start a line at top level (no indentation)")
	}
}
