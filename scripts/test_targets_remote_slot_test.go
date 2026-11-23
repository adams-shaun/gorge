package scripts

// The remote runner works in one reused worktree per run slot (tt-wt/.slot<i>.wt), not a fresh one per run: go's test
// cache keys PWD and the absolute path and mtime of every module file a test opens, so a fresh worktree never hits.
// These tests pin the reuse, that an unchanged file keeps its mtime, that the slot's flock keeps two runs apart,
// and that a run killed mid-checkout does not wedge the slot.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// goTestDirs: the directory each go test ran in, in order (the stub go appends `<mark> <pwd>` to MARKS).
func (r *rig) goTestDirs() []string {
	b, _ := os.ReadFile(filepath.Join(filepath.Dir(r.repo), "marks"))
	var d []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			d = append(d, f[len(f)-1])
		}
	}
	return d
}

func (r *rig) mustPass(what string) {
	r.t.Helper()
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(r.t, out)[0], "test-one: PASS ") || !strings.Contains(out, "== remote") {
		r.t.Fatalf("%s: exit %d\n%s\n%s", what, code, out, errs)
	}
	if strings.Contains(errs, "running locally") {
		r.t.Fatalf("%s fell back to a local run:\n%s", what, errs)
	}
}

func (r *rig) commit(file, content string) {
	r.t.Helper()
	write(r.t, filepath.Join(r.repo, file), content, 0o644)
	sh(r.t, r.repo, "git", "add", "-A")
	sh(r.t, r.repo, "git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", file)
}

func mtime(t *testing.T, p string) time.Time {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.ModTime()
}

func TestTestTargetsRemoteReusesTheSlotWorktree(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	slot := filepath.Join(r.box, "tt-wt", ".slot1.wt")
	r.commit("other.txt", "v1\n")
	r.mustPass("first run")
	keep := mtime(t, filepath.Join(slot, "tracked.txt"))
	write(t, filepath.Join(slot, "litter.txt"), "a previous run's untracked file\n", 0o644)
	write(t, filepath.Join(slot, ".ds4", "scratch", "test", "one-stale.json"), "{}\n", 0o644)

	// A rewritten file must get a later mtime than keep: wait until this filesystem's clock has
	// ticked past it, by writing a probe file there, not by sleeping a fixed wall-clock window.
	for probe := filepath.Join(r.box, "mtime-probe"); ; {
		write(t, probe, "tick\n", 0o644)
		if mtime(t, probe).After(keep) {
			break
		}
		runtime.Gosched()
	}
	r.commit("other.txt", "v2\n")
	write(t, filepath.Join(r.repo, "fresh.txt"), "uncommitted, shipped as the patch\n", 0o644)
	r.mustPass("second run")

	if d := r.goTestDirs(); len(d) != 2 || d[0] != slot || d[1] != slot {
		t.Fatalf("go test ran in %q, want both runs in %s", d, slot)
	}
	if b, _ := os.ReadFile(filepath.Join(slot, "other.txt")); string(b) != "v2\n" {
		t.Errorf("the slot is not at the second run's head: other.txt = %q", b)
	}
	if got := mtime(t, filepath.Join(slot, "tracked.txt")); !got.Equal(keep) {
		t.Errorf("an unchanged file was rewritten (mtime %v -> %v): its cached test results would miss", keep, got)
	}
	if _, err := os.Stat(filepath.Join(slot, "litter.txt")); !os.IsNotExist(err) {
		t.Errorf("the previous run's untracked file survived: %v", err)
	}
	// "."+"cards": codeshape's corpus-lookup ratchet flags a .cards literal beside os.Stat; this is the slot's link,
	// not a corpus lookup.
	if _, err := os.Stat(filepath.Join(slot, "."+"cards", "cardsfolder")); err != nil {
		t.Errorf("the slot lost its .cards link: %v", err)
	}
	// Only this run's results came back.
	if _, err := os.Stat(filepath.Join(r.repo, ".ds4", "scratch", "test", "one-stale.json")); !os.IsNotExist(err) {
		t.Errorf("a previous run's result came back with this one: %v", err)
	}
	r.boxIsClean()
}

// The slot's flock is what keeps two runs apart: with slot 1 held, the run takes slot 2 and its own worktree.
func TestTestTargetsRemoteHeldSlotIsNeverShared(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.set("GORGE_REMOTE_MAX_RUNS=2")
	r.mustPass("warm slot 1")
	f, err := os.OpenFile(filepath.Join(r.box, "tt-wt", ".slot1"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	busy := mtime(t, filepath.Join(r.box, "tt-wt", ".slot1.wt", "tracked.txt"))
	r.commit("other.txt", "while slot 1 is busy\n")
	r.mustPass("run beside a held slot")
	d := r.goTestDirs()
	if want := filepath.Join(r.box, "tt-wt", ".slot2.wt"); len(d) != 2 || d[1] != want {
		t.Fatalf("go test ran in %q, want the second run in %s", d, want)
	}
	if _, err := os.Stat(filepath.Join(r.box, "tt-wt", ".slot1.wt", "other.txt")); !os.IsNotExist(err) {
		t.Errorf("the held slot's worktree was checked out by another run")
	}
	if got := mtime(t, filepath.Join(r.box, "tt-wt", ".slot1.wt", "tracked.txt")); !got.Equal(busy) {
		t.Errorf("the held slot's worktree was touched")
	}
}

// A run killed mid-checkout leaves the index lock and a half-written tree; one killed inside `worktree add` leaves no
// .git file. Either way the next run in the slot recovers and runs the right head.
func TestTestTargetsRemoteInterruptedSlotRecovers(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, slot string){
		"index lock and a dirty tree": func(t *testing.T, slot string) {
			gd := strings.TrimSpace(sh(t, slot, "git", "rev-parse", "--absolute-git-dir"))
			write(t, filepath.Join(gd, "index.lock"), "", 0o644)
			write(t, filepath.Join(slot, "tracked.txt"), "half-written\n", 0o644)
		},
		"no .git file": func(t *testing.T, slot string) {
			if err := os.Remove(filepath.Join(slot, ".git")); err != nil {
				t.Fatal(err)
			}
		},
		"slot directory gone": func(t *testing.T, slot string) {
			if err := os.RemoveAll(slot); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			slot := filepath.Join(r.box, "tt-wt", ".slot1.wt")
			r.mustPass("first run")
			breakIt(t, slot)
			r.commit("other.txt", "after the crash\n")
			r.mustPass("run after the crash")
			if d := r.goTestDirs(); len(d) != 2 || d[1] != slot {
				t.Fatalf("go test ran in %q, want %s", d, slot)
			}
			for f, want := range map[string]string{"tracked.txt": "committed\n", "other.txt": "after the crash\n"} {
				if b, _ := os.ReadFile(filepath.Join(slot, f)); string(b) != want {
					t.Errorf("%s = %q, want %q", f, b, want)
				}
			}
			r.boxIsClean()
		})
	}
}
