package precheckgate

// pr-regen.sh rewrites remspan.tsv whenever it re-pins kgen's acceptance
// tables (TestRememberedSpanCensus rides with them under KGEN_REPIN=1) and
// stages only the files agentctl named. agentctl then folds pr-derived.sh's
// changes in with `git add -u`, so a derived record that is dirty in the
// working tree but unchanged in HEAD must still be reset to BASE's copy:
// otherwise the amended commit carries the rewrite and pr-precheck's --check
// flags it stale (it compares the commits, so it cannot see the working-tree
// rewrite pr-derived missed).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDerivedResetsAnUncommittedRecord: remspan.tsv differs from BASE only in
// the working tree, as after pr-regen. pr-derived.sh must notice it, restore
// BASE's copy (the tolerant test runs and passes on it), and leave the tree
// clean.
func TestDerivedResetsAnUncommittedRecord(t *testing.T) {
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
	const base = "span\tcount\nALL\t10\n"
	const rewritten = "span\tcount\nALL\t12\n"
	mk("dirac/kgen/testdata/remspan.tsv", base)
	// The other records are unchanged; only remspan is dirty, as after pr-regen.
	mk("dirac/docs/support.md", "page\n")
	mk("dirac/kgen/testdata/acceptance.tsv", "list\taccepted\tcards\n")
	mk("dirac/kgen/testdata/build_acceptance.tsv", "list\taccepted\tcards\n")
	mk("scripts/pr-cards.sh", "") // the loader the groups source; a stub keeps the repo corpus-free
	mk("scripts/pr-derived.sh", string(script))
	run("add", "-A")
	run("commit", "-qm", "base")
	mk("dirac/kgen/testdata/remspan.tsv", rewritten)
	// Precondition: BASE and HEAD still hold the old copy, and the new one is
	// only in the working tree.
	head, err := exec.Command("git", "-C", dir, "show", "HEAD:dirac/kgen/testdata/remspan.tsv").Output()
	if err != nil || string(head) != base {
		t.Fatalf("HEAD copy must hold %q, got %q (%v)", base, head, err)
	}
	if err := exec.Command("git", "-C", dir, "diff", "--quiet", "--", "dirac/kgen/testdata/remspan.tsv").Run(); err == nil {
		t.Fatal("precondition: remspan.tsv must be dirty in the working tree")
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
	// `go` behind the cap: record the call and pass.
	stub("go", "#!/usr/bin/env bash\necho \"go $*\" >> \"$GO_CALLS\"\nexit 0\n")
	// systemd-run: skip its options and the cap's VAR=val assignments (bash
	// exec takes no leading assignments), then run the trailing command.
	stub("systemd-run", "#!/usr/bin/env bash\nfor a in \"$@\"; do case $a in -*) shift ;; *=*) shift ;; *) break ;; esac; done\nexec \"$@\"\n")
	cmd := exec.Command("bash", "scripts/pr-derived.sh", "main")
	cmd.Dir = dir
	env := os.Environ()[:0]
	for _, kv := range os.Environ() { // replace PATH, so the stub bin/ is found before the real go
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GO_CALLS="+calls)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pr-derived.sh: %v\n%s", err, out)
	}
	// The handler ran: the rewrite was judged, not skipped. An absent file
	// means the gate never reached a group (the old HEAD-only comparison).
	got, _ := os.ReadFile(calls)
	if !strings.Contains(string(got), "TestRememberedSpanCensus") {
		t.Errorf("the remspan test did not run; calls:\n%s", got)
	}
	// The working-tree rewrite is gone, back at BASE's copy.
	b, err := os.ReadFile(filepath.Join(dir, "dirac/kgen/testdata/remspan.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != base {
		t.Errorf("remspan.tsv = %q, want BASE's %q", b, base)
	}
	if err := exec.Command("git", "-C", dir, "diff", "--quiet", "--", "dirac/kgen/testdata/remspan.tsv").Run(); err != nil {
		t.Error("remspan.tsv is still dirty after pr-derived.sh")
	}
}
