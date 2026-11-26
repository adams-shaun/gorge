package precheckgate

// The derived records (support.md, kgen's pin tables, remspan.tsv) rise through one regen/main
// PR, and pr-derived.sh leaves a ticket's rise at BASE's copy — but nothing verified it ran, so a branch that
// skipped it carried the rise into its PR and conflicted with regen/main the moment it merged (11-26: 25 open
// wt/* PRs CONFLICTING). These tests pin the new plumbing: pr-derived.sh --list (the one home of the four paths),
// pr-precheck.sh's derived_exempt() gate, and pr-derived.sh --check, which flags a touched record as stale exactly
// when the tolerant test passes on BASE's copy (a rise regen/main owns) and keeps a fall.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var derivedPaths = []string{
	"dirac/docs/support.md",
	"dirac/kgen/testdata/acceptance.tsv",
	"dirac/kgen/testdata/build_acceptance.tsv",
	"dirac/kgen/testdata/remspan.tsv",
}

func repoRun(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func TestDerivedListIsTheFourRecords(t *testing.T) {
	cmd := exec.Command("bash", "scripts/pr-derived.sh", "--list")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pr-derived.sh --list: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	want := append([]string(nil), derivedPaths...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("--list printed:\n%s\nwant exactly:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, p := range derivedPaths {
		if tracked := repoRun(t, "ls-files", "--error-unmatch", p); tracked == "" {
			t.Errorf("%s is not tracked: %q", p, tracked)
		}
	}
}

// TestDerivedListHasOneHome: the four paths must be declared once, in pr-derived.sh's derived array; a second copy
// of a path elsewhere in the script (or built by hand again) is where the list drifts.
func TestDerivedListHasOneHome(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-derived.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var body []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			body = append(body, line)
		}
	}
	nonComment := strings.Join(body, "\n")
	for _, p := range derivedPaths {
		if n := strings.Count(nonComment, p); n != 1 {
			t.Errorf("%s appears %d times outside comments in pr-derived.sh, want exactly once (in derived=)", p, n)
		}
	}
	decl := regexp.MustCompile(`(?m)^derived=\(.*\)$`).FindString(nonComment)
	if decl == "" {
		t.Fatal("pr-derived.sh has no derived=(...) declaration line")
	}
	for _, p := range derivedPaths {
		if !strings.Contains(decl, p) {
			t.Errorf("the derived=(...) line does not name %s", p)
		}
	}
}

func TestDerivedExempt(t *testing.T) {
	gate := scriptGate(t, "derived_exempt")
	for _, branch := range []string{"regen/main", "regen/ceilings", "mergify/merge-queue/abc"} {
		if !gate(branch + "\n") {
			t.Errorf("derived_exempt rejected %q, want exempt", branch)
		}
	}
	for _, branch := range []string{"wt/foo", "regen", "hotfix/x", "HEAD", "xregen/main"} {
		if gate(branch + "\n") {
			t.Errorf("derived_exempt exempted %q, want not exempt", branch)
		}
	}
}

// TestDerivedCheckFlagsARiseAndKeepsAFall: --check in a throwaway git repo whose `go` (behind a systemd-run stub)
// exits by STUB_GO_RC. Passing on BASE's copy is a rise: exit 1 naming the file, and the file is still HEAD's
// content afterwards. Failing is a fall: exit 0. A diff touching none of the four records runs no test at all.
func TestDerivedCheckFlagsARiseAndKeepsAFall(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	mk := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pr-derived.sh"))
	if err != nil {
		t.Fatal(err)
	}
	mk("dirac/kgen/testdata/remspan.tsv", "span\tcount\nALL\t10\n")
	mk("scripts/pr-cards.sh", "") // the loader --check's groups source; a stub keeps the repo corpus-free
	mk("scripts/pr-derived.sh", string(script))
	run("add", "-A")
	run("commit", "-qm", "base")
	run("checkout", "-q", "-b", "ticket")
	mk("dirac/kgen/testdata/remspan.tsv", "span\tcount\nALL\t12\n")
	run("commit", "-aqm", "raise")
	// Precondition: the copies --check compares really differ.
	if base, err := exec.Command("git", "-C", dir, "show", "main:dirac/kgen/testdata/remspan.tsv").Output(); err != nil || strings.Contains(string(base), "12") {
		t.Fatalf("base copy must hold 10, got %q (%v)", base, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "dirac/kgen/testdata/remspan.tsv")); err != nil || string(got) != "span\tcount\nALL\t12\n" {
		t.Fatalf("HEAD copy must hold 12, got %q (%v)", got, err)
	}

	bin := filepath.Join(dir, "bin")
	calls := filepath.Join(dir, "stub.calls")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := func(name, body string) {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// `go` behind the cap: record the call, exit by STUB_GO_RC.
	stub("go", "#!/usr/bin/env bash\necho \"go $*\" >> \"$GO_CALLS\"\nexit ${STUB_GO_RC:-1}\n")
	// systemd-run: skip its options and the cap's VAR=val assignments (bash exec takes no leading assignments),
	// then run the trailing command.
	stub("systemd-run", "#!/usr/bin/env bash\necho \"systemd-run $*\" >> \"$GO_CALLS\"\nfor a in \"$@\"; do case $a in -*) shift ;; *=*) shift ;; *) break ;; esac; done\nexec \"$@\"\n")
	check := func(rc string) (string, int) {
		cmd := exec.Command("bash", "scripts/pr-derived.sh", "--check", "main")
		cmd.Dir = dir
		env := os.Environ()[:0]
		for _, kv := range os.Environ() { // replace PATH, so the stub bin/ is found before the real go
			if !strings.HasPrefix(kv, "PATH=") {
				env = append(env, kv)
			}
		}
		cmd.Env = append(env,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"GO_CALLS="+calls, "STUB_GO_RC="+rc)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("pr-derived.sh --check: %v\n%s", err, out)
		}
		return string(out), code
	}
	content := func() string {
		b, err := os.ReadFile(filepath.Join(dir, "dirac/kgen/testdata/remspan.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const raised = "span\tcount\nALL\t12\n"

	// A rise: BASE's copy is tolerated, so the branch's is stale.
	out, code := check("0")
	if code != 1 || !strings.Contains(out, "pr-derived: stale") || !strings.Contains(out, "dirac/kgen/testdata/remspan.tsv") {
		t.Errorf("rise: want exit 1 with a stale line naming the file, got exit %d, output:\n%s", code, out)
	}
	if got := content(); got != raised {
		t.Errorf("rise: --check must leave HEAD's copy in place, file is now %q", got)
	}

	// A fall: the tolerant check fails on BASE's copy, the branch keeps its copy, no complaint.
	out, code = check("1")
	if code != 0 {
		t.Errorf("fall: want exit 0, got exit %d, output:\n%s", code, out)
	}
	if got := content(); got != raised {
		t.Errorf("fall: the branch's copy must survive, file is now %q", got)
	}

	// A diff touching none of the four records exits 0 without running any test: a branch off main whose only
	// change is unrelated.
	run("checkout", "-q", "-b", "other", "main")
	mk("scripts/notes.txt", "unrelated\n")
	run("add", "-A")
	run("commit", "-qm", "unrelated")
	os.Remove(calls)
	if out, code := check("0"); code != 0 {
		t.Errorf("no derived diff: want exit 0, got exit %d, output:\n%s", code, out)
	}
	if b, err := os.ReadFile(calls); err == nil {
		t.Errorf("no derived diff: the stub was called, want no test run:\n%s", b)
	}
}
