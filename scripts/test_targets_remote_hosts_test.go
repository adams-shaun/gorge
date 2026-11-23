package scripts

// Several boxes behind the remote runner (scripts/test-remote.sh, GORGE_REMOTE_HOSTS): each run probes every box
// at once and goes to one with a free run slot, it waits only when every box is full, a box whose probe fails is
// skipped, and a box's run slots follow its cores (max(2, nproc/6)) when GORGE_REMOTE_MAX_RUNS is unset.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// stubSSHHosts is stubSSH for several boxes: the token HOSTBOX in the command (GORGE_REMOTE_REPO holds it) becomes
// box-<host>, so each host is its own temp directory. A host named in STUB_DOWN_HOSTS is unreachable.
const stubSSHHosts = `#!/bin/bash
while [ $# -gt 0 ]; do
  case $1 in -i|-o|-F|-p|-l) shift 2 ;; -*) shift ;; *) break ;; esac
done
h=${1#*@}
shift
case " $STUB_DOWN_HOSTS " in *" $h "*) echo "ssh: connect to host $h port 22: Connection timed out" >&2; exit 255 ;; esac
c="$*"
exec bash -c "${c//HOSTBOX/box-$h}"
`

// newHostsRig is newRig with the boxes box-a and box-b, reached as user@a and user@b through GORGE_REMOTE_HOSTS.
func newHostsRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t)
	root := filepath.Dir(r.box)
	for _, h := range []string{"a", "b"} {
		d := filepath.Join(root, "box-"+h)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		sh(t, d, "git", "init", "-q", "--bare", "repo.git")
	}
	write(t, filepath.Join(r.bin, "ssh"), stubSSHHosts, 0o755)
	var kept []string
	for _, kv := range r.env {
		if !strings.HasPrefix(kv, "GORGE_REMOTE_HOST=") {
			kept = append(kept, kv)
		}
	}
	r.env = append(kept, "GORGE_REMOTE_HOSTS=user@a user@b", "GORGE_REMOTE_REPO="+filepath.Join(root, "HOSTBOX", "repo.git"))
	return r
}

func (r *rig) hostBox(h string) string { return filepath.Join(filepath.Dir(r.box), "box-"+h) }

// holdSlotOn takes run slot i of the box at dir until the test ends.
func holdSlotOn(t *testing.T, dir string, i int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "tt-wt"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "tt-wt", ".slot"+string(rune('0'+i))), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
}

// ranOn says the box's go test ran in a slot worktree of box-<h>, and nowhere else.
func (r *rig) ranOn(h string) bool {
	b, _ := os.ReadFile(filepath.Join(filepath.Dir(r.repo), "marks"))
	return strings.Count(string(b), "\n") == 1 && strings.Contains(string(b), string(filepath.Separator)+"box-"+h+string(filepath.Separator))
}

func (r *rig) hostIsClean(h string) {
	r.t.Helper()
	saved := r.box
	r.box = r.hostBox(h)
	defer func() { r.box = saved }()
	r.boxIsClean()
}

func TestTestTargetsRemoteHostsFirstFullGoesToTheSecond(t *testing.T) {
	t.Parallel()
	r := newHostsRig(t)
	r.set("GORGE_REMOTE_MAX_RUNS=1", "GORGE_REMOTE_WAIT=60", "GORGE_REMOTE_POLL=1")
	holdSlotOn(t, r.hostBox("a"), 1)
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(out, "== remote: user@b,") || strings.Contains(errs, "waiting for a slot") || strings.Contains(errs, "running locally") {
		t.Errorf("a full first box must send the run to the second at once:\n%s\n%s", out, errs)
	}
	if !r.ranOn("b") {
		b, _ := os.ReadFile(filepath.Join(filepath.Dir(r.repo), "marks"))
		t.Errorf("go test did not run once on box b: %q", b)
	}
	r.hostIsClean("a")
	r.hostIsClean("b")
}

func TestTestTargetsRemoteHostsAllFullWaitThenFallBack(t *testing.T) {
	t.Parallel()
	r := newHostsRig(t)
	seen := fallbackStubGo(t, r)
	r.set("GORGE_REMOTE_MAX_RUNS=1", "GORGE_REMOTE_WAIT=2", "GORGE_REMOTE_POLL=1")
	holdSlotOn(t, r.hostBox("a"), 1)
	holdSlotOn(t, r.hostBox("b"), 1)
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "busy: every box is full") || !strings.Contains(errs, "waiting for a slot") ||
		!strings.Contains(errs, "running locally") || strings.Contains(out, "== remote") {
		t.Errorf("expected a wait on both full boxes, then a run here:\n%s\n%s", out, errs)
	}
	if b, _ := os.ReadFile(seen); string(b) != "fallback=1 remote=\n" {
		t.Errorf("the run here must carry the fallback cap marker: %q", b)
	}
}

func TestTestTargetsRemoteHostsProbeFailureIsSkipped(t *testing.T) {
	t.Parallel()
	r := newHostsRig(t)
	r.set("STUB_DOWN_HOSTS=a")
	out, errs, code := r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(out, "== remote: user@b,") || strings.Contains(errs, "running locally") || !r.ranOn("b") {
		t.Errorf("an unreachable box must be skipped, not fatal:\n%s\n%s", out, errs)
	}
	r.hostIsClean("b")

	// Every box down: no wait (nothing is busy), straight to a run here.
	r = newHostsRig(t)
	r.set("STUB_DOWN_HOSTS=a b", "GORGE_REMOTE_WAIT=60", "GORGE_REMOTE_POLL=1")
	out, errs, code = r.run("one")
	if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "no box will take the run") || !strings.Contains(errs, "unreachable") ||
		!strings.Contains(errs, "running locally") || strings.Contains(errs, "waiting for a slot") {
		t.Errorf("every box down must fall back at once:\n%s", errs)
	}
}

// With GORGE_REMOTE_MAX_RUNS unset, a box has max(2, nproc/6) run slots.
func TestTestTargetsRemoteDefaultSlotsFollowCores(t *testing.T) {
	t.Parallel()
	cases := []struct {
		nproc  string
		held   int
		remote bool
		note   string
	}{
		{"32", 4, true, ""}, // 5 slots: the fifth is free
		{"32", 5, false, "busy: all 5 remote run slots are taken"},
		{"12", 2, false, "busy: all 2 remote run slots are taken"},
		{"6", 1, true, ""}, // at least 2
	}
	for _, tc := range cases {
		t.Run(tc.nproc+"cores-"+string(rune('0'+tc.held))+"held", func(t *testing.T) {
			r := newRig(t)
			write(t, filepath.Join(r.bin, "nproc"), "#!/bin/sh\necho \"$STUB_NPROC\"\n", 0o755)
			r.set("STUB_NPROC=" + tc.nproc)
			for i := 1; i <= tc.held; i++ {
				holdSlotOn(t, r.box, i)
			}
			out, errs, code := r.run("one")
			if code != 0 || !strings.HasPrefix(verdictLines(t, out)[0], "test-one: PASS ") {
				t.Fatalf("exit %d\n%s\n%s", code, out, errs)
			}
			if got := strings.Contains(out, "== remote: ubuntu@box,"); got != tc.remote {
				t.Errorf("remote=%v, want %v:\n%s\n%s", got, tc.remote, out, errs)
			}
			if tc.note != "" && !strings.Contains(errs, tc.note) {
				t.Errorf("want note %q:\n%s", tc.note, errs)
			}
		})
	}
}

// GORGE_REMOTE_HOST alone is the one-box form; set beside GORGE_REMOTE_HOSTS, it wins (the burst box override).
func TestTestTargetsRemoteHostAloneStillWorks(t *testing.T) {
	t.Parallel()
	r := newHostsRig(t)
	var kept []string
	for _, kv := range r.env {
		if !strings.HasPrefix(kv, "GORGE_REMOTE_HOSTS=") {
			kept = append(kept, kv)
		}
	}
	r.env = append(kept, "GORGE_REMOTE_HOST=user@b")
	out, errs, code := r.run("one")
	if code != 0 || !strings.Contains(out, "== remote: user@b,") || !r.ranOn("b") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}

	r = newHostsRig(t)
	r.set("GORGE_REMOTE_HOST=user@a", "STUB_DOWN_HOSTS=b")
	out, errs, code = r.run("one")
	if code != 0 || !strings.Contains(out, "== remote: user@a,") || !r.ranOn("a") {
		t.Fatalf("GORGE_REMOTE_HOST must win over GORGE_REMOTE_HOSTS: exit %d\n%s\n%s", code, out, errs)
	}
}
