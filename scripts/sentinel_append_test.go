package scripts

// scripts/sentinel-append.sh, run against a temp tree: the auto-revert's sentinel commit appends only failing
// tests that are declared and not yet listed, lightest first under the cap, and a usage error exits 2.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sentinelAppendBase = "# header\n\n# Named by old\nTestListed\n"

// sentinelTree writes a temp repo root holding sentinels.txt, ci-shard-seconds.txt, one oracle test file
// declaring decl, a failing file and a copy of the real script.
func sentinelTree(t *testing.T, failing, seconds string, decl ...string) string {
	t.Helper()
	root := t.TempDir()
	script, err := os.ReadFile("sentinel-append.sh")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "scripts", "sentinel-append.sh"), string(script), 0o755)
	write(t, filepath.Join(root, "dirac/oracle/testdata/sentinels.txt"), sentinelAppendBase, 0o644)
	write(t, filepath.Join(root, "dirac/oracle/testdata/ci-shard-seconds.txt"), "# <test> <seconds>\n"+seconds, 0o644)
	var src strings.Builder
	src.WriteString("package oracle\n\nimport \"testing\"\n\n")
	for _, d := range decl {
		src.WriteString("func " + d + "(t *testing.T) {}\n\n")
	}
	write(t, filepath.Join(root, "dirac/oracle/a_test.go"), src.String(), 0o644)
	write(t, filepath.Join(root, "failing.txt"), failing, 0o644)
	return root
}

func runSentinelAppend(t *testing.T, root string, env []string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command("bash", append([]string{"scripts/sentinel-append.sh"}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), env...)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func sentinelsText(t *testing.T, root string) string {
	t.Helper()
	return readText(t, filepath.Join(root, "dirac/oracle/testdata/sentinels.txt"))
}

func TestSentinelAppend(t *testing.T) {
	t.Parallel()

	t.Run("appends-new-declared-names", func(t *testing.T) {
		t.Parallel()
		root := sentinelTree(t, "TestBravo\nTestAlpha\n", "", "TestAlpha", "TestBravo")
		out, code := runSentinelAppend(t, root, nil, "failing.txt", "n")
		if code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		if want := sentinelAppendBase + "\n# Named by n\nTestAlpha\nTestBravo\n"; sentinelsText(t, root) != want {
			t.Errorf("sentinels.txt:\n%s\nwant:\n%s", sentinelsText(t, root), want)
		}
		if !strings.Contains(out, "sentinel-append: added 2 of 2 (TestAlpha TestBravo)") {
			t.Errorf("output: %s", out)
		}
	})

	t.Run("skips-listed-and-gone", func(t *testing.T) {
		t.Parallel()
		root := sentinelTree(t, "TestListed\nTestGone\n", "", "TestListed", "TestOther")
		out, code := runSentinelAppend(t, root, nil, "failing.txt", "n")
		if code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		if !strings.Contains(out, "sentinel-append: nothing to add (2 failing, 1 listed, 1 gone)") {
			t.Errorf("output: %s", out)
		}
		if got := sentinelsText(t, root); got != sentinelAppendBase {
			t.Errorf("sentinels.txt changed:\n%s", got)
		}
	})

	t.Run("cap-keeps-lightest", func(t *testing.T) {
		t.Parallel()
		names := []string{"TestHeavy", "TestMid", "TestUc", "TestUa", "TestUb"}
		root := sentinelTree(t, strings.Join(names, "\n")+"\n", "TestHeavy 9\nTestMid 2\nTestNotFailing 0.1\n", names...)
		out, code := runSentinelAppend(t, root, []string{"SENTINEL_APPEND_MAX=3"}, "failing.txt", "n")
		if code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		if want := sentinelAppendBase + "\n# Named by n\nTestUa\nTestUb\nTestUc\n"; sentinelsText(t, root) != want {
			t.Errorf("sentinels.txt:\n%s\nwant:\n%s", sentinelsText(t, root), want)
		}
		for _, want := range []string{
			"sentinel-append: added 3 of 5 (TestUa TestUb TestUc)",
			"sentinel-append: over the cap of 3, not added: TestMid TestHeavy",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
	})

	t.Run("ignores-non-test-lines", func(t *testing.T) {
		t.Parallel()
		root := sentinelTree(t, "ok\n--- FAIL: TestAlpha (0.1s)\n\n  TestAlpha\nTestAlpha extra\nFAIL\n", "", "TestAlpha")
		out, code := runSentinelAppend(t, root, nil, "failing.txt", "n")
		if code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		if !strings.Contains(out, "sentinel-append: nothing to add (0 failing, 0 listed, 0 gone)") {
			t.Errorf("output: %s", out)
		}
		if got := sentinelsText(t, root); got != sentinelAppendBase {
			t.Errorf("sentinels.txt changed:\n%s", got)
		}
	})

	t.Run("invalid-max-exit-2", func(t *testing.T) {
		t.Parallel()
		root := sentinelTree(t, "TestAlpha\n", "", "TestAlpha")
		for _, v := range []string{"0", "08", "x", "-1"} {
			out, code := runSentinelAppend(t, root, []string{"SENTINEL_APPEND_MAX=" + v}, "failing.txt", "n")
			if code != 2 {
				t.Errorf("SENTINEL_APPEND_MAX=%s: exit %d, want 2\n%s", v, code, out)
			}
			if got := sentinelsText(t, root); got != sentinelAppendBase {
				t.Errorf("SENTINEL_APPEND_MAX=%s changed sentinels.txt:\n%s", v, got)
			}
		}
	})

	t.Run("usage-exit-2", func(t *testing.T) {
		t.Parallel()
		root := sentinelTree(t, "TestAlpha\n", "", "TestAlpha")
		for _, args := range [][]string{nil, {"failing.txt"}, {"missing.txt", "n"}} {
			out, code := runSentinelAppend(t, root, nil, args...)
			if code != 2 {
				t.Errorf("args %q: exit %d, want 2\n%s", args, code, out)
			}
		}
		if err := os.Remove(filepath.Join(root, "dirac/oracle/testdata/sentinels.txt")); err != nil {
			t.Fatal(err)
		}
		if out, code := runSentinelAppend(t, root, nil, "failing.txt", "n"); code != 2 {
			t.Errorf("no sentinels.txt: exit %d, want 2\n%s", code, out)
		}
	})
}
