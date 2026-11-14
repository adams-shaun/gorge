package manifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnv isolates the fixture repositories from the user's and system git
// configuration and pins their dates.
var gitEnv = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture",
	"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture",
	"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
}

func gitT(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), gitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// landRepo is a git repository holding testdata/tree with every next entry
// assigned, committed on branch land and checked out there.
func landRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := copyTree(t)
	gitT(t, root, "init", "-q", "-b", "land")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", "fixture")
	base, err := ReadGit(root, "land")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Assign(root, base); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "manifest: assign")
	return root
}

func commitAll(t *testing.T, root, msg string) {
	t.Helper()
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", msg)
}

func writeEntry(t *testing.T, root, p, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assign(t *testing.T, root, ref string) []Assignment {
	t.Helper()
	base, err := ReadGit(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Assign(root, base)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestAssignWritesOnlyTheValueLine: the fixture's next entries get the values
// their provisional ones were, and each file changes in its value line alone.
func TestAssignWritesOnlyTheValueLine(t *testing.T) {
	root := landRepo(t)
	want := map[string]string{
		"CounterKind/CtrVoyage": "5", "ExtOp/ExtFoo": "2", "Op/OpHot": "158",
		"TrigFlags2/Tf2New": "2", "mech/mechFooBar": "55",
	}
	for p, v := range want {
		before := readFileAt(t, root, "land~1", Dir+"/"+p)
		after := readFile(t, root, Dir+"/"+p)
		if strings.Replace(before, "value: next\n", "value: "+v+"\n", 1) != after {
			t.Errorf("%s:\n%s\nwant the value line set to %s and nothing else; before:\n%s", p, after, v, before)
		}
	}
	if got := assign(t, root, "land"); len(got) != 0 {
		t.Errorf("a second assign wrote %v", got)
	}
}

func readFileAt(t *testing.T, root, ref, p string) string {
	t.Helper()
	return gitT(t, root, "show", ref+":"+p) + "\n"
}

// TestAssignIsSerialAgainstLand: two units that each took the same
// provisional value land one after the other and end with distinct values;
// and a unit not yet rebased past an entry land took cannot take its value.
func TestAssignIsSerialAgainstLand(t *testing.T) {
	root := landRepo(t)
	unitA := "family: ExtOp\nname: ExtUnitA\nvalue: next\nstatus: active\n"
	unitB := "family: ExtOp\nname: ExtUnitB\nvalue: next\nstatus: active\n"

	// Each unit alone, on land: both provisionally take 4 (0 1 3 are hand
	// constants, 2 is ExtFoo and 5 ExtBar).
	for _, u := range []struct{ p, body string }{{Dir + "/ExtOp/ExtUnitA", unitA}, {Dir + "/ExtOp/ExtUnitB", unitB}} {
		writeEntry(t, root, u.p, u.body)
		tr := load(t, os.DirFS(root))
		if v := tr.Entry(u.p).Value; v != 4 {
			t.Errorf("%s provisional %d, want 4", u.p, v)
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(u.p))); err != nil {
			t.Fatal(err)
		}
	}

	// Land A, then B on top of it.
	writeEntry(t, root, Dir+"/ExtOp/ExtUnitA", unitA)
	if got := assign(t, root, "land"); len(got) != 1 || got[0].Value != 4 {
		t.Fatalf("unit A: %v, want ExtUnitA = 4", got)
	}
	commitAll(t, root, "unit A")
	writeEntry(t, root, Dir+"/ExtOp/ExtUnitB", unitB)
	if got := assign(t, root, "land"); len(got) != 1 || got[0].Value != 6 {
		t.Fatalf("unit B: %v, want ExtUnitB = 6", got)
	}
	commitAll(t, root, "unit B")

	// A unit whose tree lacks ExtUnitA and ExtUnitB (not rebased) still
	// steps past their values, which land holds.
	for _, n := range []string{"ExtUnitA", "ExtUnitB"} {
		if err := os.Remove(filepath.Join(root, Dir, "ExtOp", n)); err != nil {
			t.Fatal(err)
		}
	}
	writeEntry(t, root, Dir+"/ExtOp/ExtUnitC", "family: ExtOp\nname: ExtUnitC\nvalue: next\nstatus: active\n")
	if got := assign(t, root, "land"); len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("unit C: %v, want ExtUnitC = 7", got)
	}
}

func TestAssignRefuses(t *testing.T) {
	cases := []struct {
		name, path, body, want string
	}{
		{"a value back to next", Dir + "/CounterKind/CtrTicket",
			"family: CounterKind\nname: CtrTicket\nvalue: next\nforge: TICKET\nstatus: reserved\n", "has value 4 at the base"},
		{"a hand constant's name", Dir + "/ExtOp/ExtA",
			"family: ExtOp\nname: ExtA\nvalue: next\nstatus: reserved\n", "ExtA is a ExtOp constant at the base"},
		{"an unapproved hot op", Dir + "/Op/OpCold",
			"family: Op\nname: OpCold\nvalue: next\nstatus: reserved\n", "add an approval line"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := landRepo(t)
			writeEntry(t, root, c.path, c.body)
			base, err := ReadGit(root, "land")
			if err != nil {
				t.Fatal(err)
			}
			_, err = Assign(root, base)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
			if got := readFile(t, root, c.path); got != c.body {
				t.Errorf("a refused assign wrote %s:\n%s", c.path, got)
			}
		})
	}
}

// TestReadGitMatchesTheWorkingTree: a commit reads as the files it holds.
func TestReadGitMatchesTheWorkingTree(t *testing.T) {
	root := landRepo(t)
	g, err := ReadGit(root, "land")
	if err != nil {
		t.Fatal(err)
	}
	w := load(t, os.DirFS(root))
	if len(g.Entries) != len(w.Entries) || len(g.Ordinals) != len(w.Ordinals) {
		t.Fatalf("git %d entries %d rows, working tree %d entries %d rows", len(g.Entries), len(g.Ordinals), len(w.Entries), len(w.Ordinals))
	}
	for i, e := range w.Entries {
		if string(g.Entries[i].raw) != string(e.raw) || g.Entries[i].Path != e.Path {
			t.Errorf("entry %d: git %s, working tree %s", i, g.Entries[i].Path, e.Path)
		}
	}
	if _, err := ReadGit(root, "no-such-ref"); err == nil {
		t.Error("ReadGit of a missing ref succeeded")
	}
}
