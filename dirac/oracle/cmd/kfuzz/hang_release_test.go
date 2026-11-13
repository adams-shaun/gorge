package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// A confirmed hang ends the segment: the feed stops, the pair after the hang is
// never played in-process (the continuation plays it), the continuation is
// asked for exactly the unplayed pairs, the hang line names the rerun command,
// and the segment still fails. That the hung game's goroutine itself is gone is
// TestContinuationExecReplacesTheImage's claim: the continuation execs.
func TestHangReleasesItsCore(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	ps := testPairs(3)
	var calls [3]atomic.Int32
	out, playedN := playAll(3, 1, time.Second, true, func(i int) played {
		calls[i].Add(1)
		if i == 1 {
			return lockstepStub(release)
		}
		return lockstepStub(nil)
	})
	if playedN != 2 {
		t.Fatalf("fed %d pairs, want 2 (the feed stopped at the hang)", playedN)
	}
	if !out[1].hang {
		t.Fatal("pair 1 was not recorded as a hang")
	}
	if n := calls[2].Load(); n != 0 {
		t.Fatalf("pair 2 played %d times in-process, want 0 (the continuation plays it)", n)
	}
	var gotFirst, gotPairs int
	var got bool
	saved := runContinuation
	runContinuation = func(first, pairs int) int { gotFirst, gotPairs, got = first, pairs, true; return 0 }
	t.Cleanup(func() { runContinuation = saved })
	o := sweepOpts{lead: "gorge", surface: "autopay", timeout: time.Second}
	code, stdout := capture(t, func() int { return finishSweep("fdn", ps, out, playedN, oracle.RunConfig{}, o, nil) })
	if code != 1 {
		t.Errorf("exit %d, want 1 (a hang fails its segment)\n%s", code, stdout)
	}
	if !got || gotFirst != ps[2].Index || gotPairs != len(ps)-playedN {
		t.Errorf("continuation (%v, pair %d, %d pairs), want pair %d, %d pairs", got, gotFirst, gotPairs, ps[2].Index, len(ps)-playedN)
	}
	if !strings.Contains(stdout, "hang: no result after two watchdogs of 1s; rerun with: ") {
		t.Errorf("the hang line names no rerun command:\n%s", stdout)
	}
}

// contArgs reruns exactly the pairs its caller names: every -pods, -first-pair
// and -pairs occurrence is dropped, in the -flag v, -flag=v and --flag v
// spellings alike, and the segment's own are appended, so anything else on the
// line is inherited.
func TestContArgsResumesTheRemainder(t *testing.T) {
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"kfuzz", "-kernel", "fdn", "-pods", "3", "--pods=6", "-seats", "4", "-first-pair", "2",
		"--first-pair", "3", "-pairs=10", "--pairs", "11", "-seed", "9", "-surface=autopay",
		"-pairs-file", "p.tsv", "-game-timeout", "30s"}
	joined := strings.Join(contArgs(5, 4), " ")
	for _, want := range []string{"-kernel fdn", "-seats 4", "-seed 9", "-surface=autopay",
		"-pairs-file p.tsv", "-game-timeout 30s", "-first-pair 5 -pairs 4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no %q in %q", want, joined)
		}
	}
	for _, bad := range []string{"-pods", "--pods", "-first-pair 2", "-first-pair 3", "-pairs=10", "-pairs 10",
		"--pairs 11", " 6", " 11"} {
		if strings.Contains(joined, bad) {
			t.Errorf("%q survived in %q", bad, joined)
		}
	}
	// The rerun command names a program a shell can run: the binary, or `go run`
	// when this is a soon-deleted go-build temp.
	os.Args = []string{"/usr/local/bin/kfuzz", "-seed", "1"}
	if got := rerunCmd(7); got != "/usr/local/bin/kfuzz -seed 1 -first-pair 7 -pairs 1" {
		t.Errorf("rerunCmd = %q", got)
	}
	os.Args = []string{"/tmp/go-build123456/b001/exe/kfuzz", "-seed", "1"}
	if got := rerunCmd(7); got != "go run ./dirac/oracle/cmd/kfuzz -seed 1 -first-pair 7 -pairs 1" {
		t.Errorf("rerunCmd under a go-build temp = %q", got)
	}
}

// A clean segment of a continuation still fails the run: a prior segment's
// hang already failed it (the exec set priorHungEnv), and the continuation's
// code cannot rescue the run.
func TestPriorHungCodeFailsACleanContinuation(t *testing.T) {
	saved, had := os.LookupEnv(priorHungEnv)
	t.Cleanup(func() {
		if had {
			os.Setenv(priorHungEnv, saved)
		} else {
			os.Unsetenv(priorHungEnv)
		}
	})
	os.Unsetenv(priorHungEnv)
	if got := priorHungCode(0); got != 0 {
		t.Errorf("clean segment without a prior hang: exit %d, want 0", got)
	}
	os.Setenv(priorHungEnv, "1")
	if got := priorHungCode(0); got != 1 {
		t.Errorf("clean segment of a continuation: exit %d, want 1", got)
	}
	if got := priorHungCode(2); got != 2 {
		t.Errorf("failed segment of a continuation: exit %d, want its own 2", got)
	}
}

// The hung segment's core is released: runContinuation execs the continuation
// into this same process, so the hung game's goroutine — which Go cannot
// cancel — is destroyed with the segment's image instead of spinning until the
// run exits, and the exec'd image fails even a clean segment (priorHungCode).
// The helper runs in this test binary: its first image spins a goroutine that
// appends to a marker file, then hands the rest to runContinuation; the exec'd
// image finds the same pid (the image was replaced, not a child spawned), sees
// priorHungEnv, and sees the marker frozen (nothing spins any more).
func TestContinuationExecReplacesTheImage(t *testing.T) {
	if os.Getenv(helperEnv) != "" {
		t.Skip("helper test")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestHangReleasesItsCoreHelper$", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), helperEnv+"=hang")
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err = cmd.Run()
	text := buf.String()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("helper: %v\n%s", err, text)
		}
		code = ee.ExitCode()
	}
	if code != 1 {
		t.Errorf("helper exit %d, want 1 (the continuation image fails a clean segment)\n%s", code, text)
	}
	if strings.Contains(text, "CONTINUATION RETURNED") {
		t.Errorf("runContinuation returned: the image was not replaced\n%s", text)
	}
	if !strings.Contains(text, helperAfterOK) {
		t.Errorf("the continuation image did not verify the release\n%s", text)
	}
}

// The helper's images: the first spins a goroutine and execs the continuation;
// the exec'd image checks the release and fails the run (a prior segment hung).
func TestHangReleasesItsCoreHelper(t *testing.T) {
	if os.Getenv(helperEnv) == "" {
		t.Skip("helper test")
	}
	if *helperCont {
		helperAfter()
		return
	}
	helperHang(t)
}

const (
	helperEnv     = "GORGE_KFUZZ_HELPER"
	helperPIDEnv  = "GORGE_KFUZZ_HELPER_PID"
	helperSpinEnv = "GORGE_KFUZZ_HELPER_SPIN"
	helperAfterOK = "HELPER-AFTER OK"
)

// helperCont marks the exec'd continuation image of TestHangReleasesItsCoreHelper.
var helperCont = flag.Bool("test.hang-helper-cont", false, "the continuation image of TestHangReleasesItsCoreHelper")

// The helper's continuation argv carries the sweep's -first-pair and -pairs
// (contArgs appends them), and the exec'd image is this test binary, which
// defines neither: the helper's images accept them. No test calls run(), so
// the sweep's own registration of the same names never collides here.
var helperSweepFlags = func() bool {
	flag.Int("first-pair", 0, "helper: runContinuation's argv")
	flag.Int("pairs", 0, "helper: runContinuation's argv")
	return true
}()

// helperHang is the hung segment's image: it spins a goroutine on a marker
// file, records this pid and the marker for the exec'd image, and hands the
// rest to runContinuation. Only an exec failure returns from that.
func helperHang(t *testing.T) {
	dir, err := os.MkdirTemp("", "kfuzz-hang-*")
	if err != nil {
		t.Fatal(err)
	}
	spin := filepath.Join(dir, "spin")
	f, err := os.OpenFile(spin, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for i := 0; ; i++ {
			fmt.Fprintf(f, "tick %d\n", i)
			time.Sleep(2 * time.Millisecond)
		}
	}()
	for spinSize(spin) == 0 {
		time.Sleep(5 * time.Millisecond) // precondition: the marker is growing before the exec
	}
	if spinSize(spin) == 0 {
		t.Fatal("the spin marker never grew")
	}
	os.Setenv(helperPIDEnv, strconv.Itoa(os.Getpid()))
	os.Setenv(helperSpinEnv, spin)
	saved := os.Args
	os.Args = append(append([]string{}, saved...), "-test.hang-helper-cont")
	code := runContinuation(5, 3)
	os.Args = saved
	os.RemoveAll(dir)
	fmt.Println("CONTINUATION RETURNED", code) // only an exec failure reaches this
}

// helperAfter is the exec'd continuation image: same pid (the image was
// replaced, not a child spawned), priorHungEnv set, and the hung game's
// goroutine is gone — the marker it was appending to is frozen. It exits
// priorHungCode(0) = 1: a clean segment of a continuation still fails the run.
func helperAfter() {
	fail := func(format string, args ...any) {
		fmt.Printf("HELPER-AFTER FAILED: "+format+"\n", args...)
		os.Exit(2)
	}
	if os.Getenv(priorHungEnv) != "1" {
		fail("no %s in the continuation image", priorHungEnv)
	}
	pid, err := strconv.Atoi(os.Getenv(helperPIDEnv))
	if err != nil || pid != os.Getpid() {
		fail("pid %d, want the hung image's %d: the image was not replaced", os.Getpid(), pid)
	}
	spin := os.Getenv(helperSpinEnv)
	before := spinSize(spin)
	if before == 0 {
		fail("the spin marker did not survive the exec")
	}
	time.Sleep(150 * time.Millisecond)
	if after := spinSize(spin); after != before {
		fail("the hung game's goroutine still runs: the spin marker grew %d -> %d bytes", before, after)
	}
	os.RemoveAll(filepath.Dir(spin))
	fmt.Println(helperAfterOK)
	os.Exit(priorHungCode(0))
}

// spinSize is the marker file's size (0: unreadable or not there yet).
func spinSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
