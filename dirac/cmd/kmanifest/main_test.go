package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// fixtureRoot copies dirac/manifest's fixture tree into a temporary module
// root.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := os.DirFS(filepath.Join("..", "..", "manifest", "testdata", "tree"))
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Execute(newRoot(), args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestGen(t *testing.T) {
	root := fixtureRoot(t)
	code, out, errs := run(t, "gen", "--root", root)
	if code != 0 {
		t.Fatalf("gen: exit %d: %s", code, errs)
	}
	want := "wrote " + strings.Join(manifest.GenFiles, "\nwrote ") + "\n"
	if out != want {
		t.Errorf("gen printed %q, want %q", out, want)
	}
	if code, out, _ = run(t, "gen", "--root", root); code != 0 || out != "" {
		t.Errorf("second gen: exit %d, printed %q; want a silent no-op", code, out)
	}
	if code, out, _ = run(t, "gen", "--check", "--root", root); code != 0 || out != "" {
		t.Errorf("gen --check on a fresh tree: exit %d, printed %q", code, out)
	}
	if err := os.WriteFile(filepath.Join(root, "dirac", "manifest", "ExtOp", "ExtBar"),
		[]byte("family: ExtOp\nname: ExtBar\nvalue: 9\nstatus: active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ = run(t, "gen", "--check", "--root", root); code != 1 || out != "stale "+manifest.KopGen+"\n" {
		t.Errorf("gen --check on a stale tree: exit %d, printed %q", code, out)
	}
}

func TestGenRefusesAnInvalidTree(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.WriteFile(filepath.Join(root, "dirac", "manifest", "Op", "OpHot"),
		[]byte("family: Op\nname: OpHot\nvalue: next\nstatus: active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errs := run(t, "gen", "--root", root)
	if code != 1 || !strings.Contains(errs, "add an approval line") {
		t.Errorf("gen on an unapproved hot op: exit %d, stderr %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(manifest.KopGen))); err == nil {
		t.Errorf("gen wrote %s from an invalid tree", manifest.KopGen)
	}
}

func TestShow(t *testing.T) {
	root := fixtureRoot(t)
	code, out, _ := run(t, "show", "--root", root, "ticket")
	if want := "dirac/manifest/CounterKind/CtrTicket value 4 reserved state code 0\n"; code != 0 || !strings.HasPrefix(out, want) {
		t.Errorf("show ticket: exit %d, printed %q; want it to start %q", code, out, want)
	}
	if code, _, errs := run(t, "show", "--root", root, "nothing"); code != 1 || !strings.Contains(errs, `no manifest entry names "nothing"`) {
		t.Errorf("show nothing: exit %d, stderr %q", code, errs)
	}
}

func gitT(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestAssignThenCheck is the landing sequence: assign against the branch
// writes the final values and regenerates, check --final passes, and a
// changed value fails check.
func TestAssignThenCheck(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := fixtureRoot(t)
	gitT(t, root, "init", "-q", "-b", "land")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", "fixture")
	if code, _, errs := run(t, "assign", "--root", root); code != 2 || !strings.Contains(errs, "--base is required") {
		t.Errorf("assign without --base: exit %d, stderr %q", code, errs)
	}
	code, out, errs := run(t, "assign", "--root", root, "--base", "land")
	if code != 0 {
		t.Fatalf("assign: exit %d: %s", code, errs)
	}
	for _, want := range []string{"assigned dirac/manifest/ExtOp/ExtFoo = 2\n", "assigned dirac/manifest/Op/OpHot = 158\n", "wrote " + manifest.KopGen + "\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("assign printed %q, want %q in it", out, want)
		}
	}
	if code, out, _ = run(t, "check", "--root", root, "--base", "land", "--final"); code != 0 {
		t.Errorf("check --final after assign: exit %d: %s", code, out)
	}
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", "manifest: assign")
	if err := os.WriteFile(filepath.Join(root, "dirac", "manifest", "ExtOp", "ExtFoo"),
		[]byte("family: ExtOp\nname: ExtFoo\nvalue: 9\nstatus: reserved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ = run(t, "check", "--root", root, "--base", "land"); code != 1 || !strings.Contains(out, "value 2 at the base is now 9") {
		t.Errorf("check on a renumbered entry: exit %d, printed %q", code, out)
	}
}

// TestSeed seeds the fixture census: a dry run writes nothing, a run writes
// the entries and prints the counts, and a second run adds nothing.
func TestSeed(t *testing.T) {
	root := fixtureRoot(t)
	fix := filepath.Join("..", "..", "manifest", "testdata", "seed")
	args := []string{"seed", "--root", root, "--census", filepath.Join(fix, "census.tsv"),
		"--classes", filepath.Join(fix, "classes.tsv"), "--aliases", filepath.Join(fix, "aliases.tsv")}
	if code, _, errs := run(t, "seed", "--root", root); code != 2 || !strings.Contains(errs, "--census and --classes are required") {
		t.Errorf("seed without inputs: exit %d, stderr %q", code, errs)
	}
	code, out, errs := run(t, append(args, "--dry-run")...)
	if code != 0 {
		t.Fatalf("seed --dry-run: exit %d: %s", code, errs)
	}
	want := "CounterKind\t4\nExtOp\t3\nKwID\t2\nTrigEvent\t1\nkey\t2\ntotal\t12\nskipped\t6\n"
	if out != want {
		t.Errorf("seed --dry-run printed %q, want %q", out, want)
	}
	if _, err := os.Stat(filepath.Join(root, "dirac", "manifest", "CounterKind", "CtrIce")); err == nil {
		t.Error("seed --dry-run wrote an entry")
	}
	if code, out, errs = run(t, args...); code != 0 || out != want {
		t.Fatalf("seed: exit %d, printed %q: %s", code, out, errs)
	}
	if code, out, errs = run(t, "show", "--root", root, "ICE"); code != 0 || !strings.HasPrefix(out, "dirac/manifest/CounterKind/CtrIce value 5 reserved state code 0\n") {
		t.Errorf("show ICE after the seed: exit %d, printed %q: %s", code, out, errs)
	}
	if code, out, _ = run(t, args...); code != 0 || !strings.HasPrefix(out, "total\t0\n") {
		t.Errorf("a second seed: exit %d, printed %q; want nothing new", code, out)
	}
}
