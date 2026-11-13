package main

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// A confirmed hang ends the segment: the pair after it is never played in this
// process (the hung game's goroutine holds a core until the process exits), the
// continuation is asked for exactly the unplayed pairs, the hang line names the
// rerun command, and the segment still fails.
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
// and -pairs occurrence is dropped, in the -flag v and -flag=v spellings alike,
// and the segment's own are appended, so anything else on the line is inherited.
func TestContArgsResumesTheRemainder(t *testing.T) {
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"kfuzz", "-kernel", "fdn", "-pods", "3", "-seats", "4", "-first-pair", "2",
		"-pairs=10", "-seed", "9", "-surface=autopay", "-pairs-file", "p.tsv", "-game-timeout", "30s"}
	joined := strings.Join(contArgs(5, 4), " ")
	for _, want := range []string{"-kernel fdn", "-seats 4", "-seed 9", "-surface=autopay",
		"-pairs-file p.tsv", "-game-timeout 30s", "-first-pair 5 -pairs 4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no %q in %q", want, joined)
		}
	}
	for _, bad := range []string{"-pods", "-first-pair 2", "-pairs=10", "-pairs 10"} {
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
