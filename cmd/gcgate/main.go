// Command gcgate runs a package's test binary under a garbage-collector
// budget and fails when the collector eats more than its share of the CPU.
//
// Why this exists: gorge's correctness gates (build, vet, tests, -race,
// mutation tests) are blind to allocation. A test that churns five gigabytes
// and a test that churns five megabytes both print "ok". A single host test
// was found allocating 5.3GB and nothing in the suite noticed, because there
// was nothing in the suite that could.
//
// # The measurement, and why the obvious one is wrong
//
// GODEBUG=gctrace=1's own leading percentage is GC CPU over GOMAXPROCS
// integrated across wall time — that is, over every core the process COULD
// have used, idle ones included. On a 32-core box running a mostly-serial
// test suite that denominator is enormous, so the figure reads ~1% no matter
// how badly the code allocates. It is not a useful budget.
//
// gcgate divides instead by CPU actually consumed (the child's user + system
// time from getrusage), which is the number a human means by "percent of CPU
// spent collecting garbage".
//
// # Idle-mark CPU counts, and why
//
// Go's collector opportunistically marks on processors the scheduler cannot
// otherwise fill ("idle mark"). "Idle" there means no runnable goroutine in
// THIS process: the runtime knows nothing about the rest of the machine. On a
// shared box — this repo's own build host runs a fleet of agents, an
// inference server and other test binaries concurrently — those cycles are
// taken straight out of whatever else is running. They are free only on an
// otherwise-empty machine, so the default budget counts them.
// -compulsory-only exists for the dedicated-runner case, and narrows what is
// measured; it does not make the excluded work stop happening.
//
// # Why GOMAXPROCS is pinned
//
// Idle-mark scales with core count, so the same binary measured 46.5% of its
// CPU in GC with 32 procs and 17.7% with 4. An unpinned budget therefore
// passes or fails on which machine ran it. Pinning makes the number a
// property of the code — it is a determinism control, not a discount.
//
// # What this does NOT catch
//
// A ratio is blind to scale: a change that doubles both real work and garbage
// leaves it flat. Pair this gate with an absolute allocation budget
// (testing.AllocsPerRun, or -benchmem deltas) — neither one subsumes the other.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	var (
		pkg      = flag.String("pkg", "./host", "package whose test binary to run")
		procs    = flag.Int("maxprocs", 4, "GOMAXPROCS for the run; pinned so the budget is a property of the code, not the box")
		budget   = flag.Float64("budget", 0.30, "maximum fraction of consumed CPU that may be spent in the garbage collector")
		memLimit = flag.String("memlimit", "5GiB", "GOMEMLIMIT for the run (FL-89)")
		run      = flag.String("run", "", "optional -test.run filter")
		compul   = flag.Bool("compulsory-only", false, "exclude idle-mark CPU, budgeting only GC work the scheduler could not avoid")
		verbose  = flag.Bool("v", false, "echo the test binary's own output")
	)
	flag.Parse()

	if err := gate(*pkg, *procs, *budget, *memLimit, *run, *compul, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "gcgate: %v\n", err)
		os.Exit(1)
	}
}

func gate(pkg string, procs int, budget float64, memLimit, run string, compulsoryOnly, verbose bool) error {
	bin, cleanup, err := build(pkg)
	if err != nil {
		return err
	}
	defer cleanup()

	args := []string{"-test.count=1"}
	if run != "" {
		args = append(args, "-test.run="+run)
	}
	cmd := exec.Command(bin, args...)
	// The test binary is built from pkg, but nothing guarantees it runs from
	// there; tests that reach for testdata rely on the package directory.
	cmd.Dir = pkg
	cmd.Env = append(os.Environ(),
		"GODEBUG=gctrace=1",
		fmt.Sprintf("GOMAXPROCS=%d", procs),
		"GOMEMLIMIT="+memLimit,
	)
	var trace strings.Builder
	cmd.Stderr = &trace // gctrace goes to stderr; the suite's own output goes to stdout
	if verbose {
		cmd.Stdout = os.Stdout
	}

	start := time.Now()
	runErr := cmd.Run()
	wall := time.Since(start)

	// Parse before reporting the test failure: a suite that fails still
	// produced a real GC trace, and the numbers help explain the failure.
	s := parse(trace.String())
	busy, ok := consumedCPU(cmd)
	if !ok {
		return fmt.Errorf("could not read the child's rusage on this platform; gcgate needs it for the denominator")
	}
	if busy <= 0 {
		return fmt.Errorf("child consumed no measurable CPU (%.3fs); nothing to budget", busy)
	}

	gc := s.total()
	label := "total GC"
	if compulsoryOnly {
		gc = s.compulsory()
		label = "compulsory GC"
	}
	share := gc / busy

	fmt.Printf("gcgate %s  GOMAXPROCS=%d  GOMEMLIMIT=%s\n", pkg, procs, memLimit)
	fmt.Printf("  wall                %8.2fs\n", wall.Seconds())
	fmt.Printf("  consumed CPU        %8.2fs   (user+sys, the denominator)\n", busy)
	fmt.Printf("  GC cycles           %8d\n", s.cycles)
	fmt.Printf("    sweep termination %8.2fs\n", s.sweep)
	fmt.Printf("    mark assist       %8.2fs\n", s.assist)
	fmt.Printf("    mark dedicated    %8.2fs\n", s.dedicated)
	fmt.Printf("    mark idle         %8.2fs   (no runnable goroutine in THIS process; still real CPU taken from the box)\n", s.idle)
	fmt.Printf("    mark termination  %8.2fs\n", s.markTerm)
	fmt.Printf("  %-17s %8.2fs   = %.1f%% of consumed CPU (budget %.1f%%)\n",
		label, gc, 100*share, 100*budget)

	if runErr != nil {
		return fmt.Errorf("test binary failed: %w (GC share was %.1f%%)", runErr, 100*share)
	}
	if s.cycles == 0 {
		return fmt.Errorf("no GC cycles observed — the gate measured nothing; check that GODEBUG reached the child")
	}
	if share > budget {
		return fmt.Errorf("%s used %.1f%% of consumed CPU, over the %.1f%% budget",
			label, 100*share, 100*budget)
	}
	return nil
}

// build compiles pkg's test binary to a temporary path and returns it with a
// cleanup. It deliberately does not reuse a cached binary: the gate must
// measure the tree as it stands.
func build(pkg string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "gcgate")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	bin := filepath.Join(dir, "pkg.test")
	cmd := exec.Command("go", "test", "-c", "-o", bin, pkg)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("building %s test binary: %w", pkg, err)
	}
	abs, err := filepath.Abs(bin)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return abs, cleanup, nil
}

// consumedCPU is the child's user+system time. ProcessState.SysUsage carries
// a *syscall.Rusage on unix; a platform where it does not is reported rather
// than silently guessed at, because a wrong denominator makes the gate lie.
func consumedCPU(cmd *exec.Cmd) (float64, bool) {
	if cmd.ProcessState == nil {
		return 0, false
	}
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0, false
	}
	sec := func(tv syscall.Timeval) float64 { return float64(tv.Sec) + float64(tv.Usec)/1e6 }
	return sec(ru.Utime) + sec(ru.Stime), true
}

// stats are GC CPU-seconds by class, summed over every cycle in a trace.
type stats struct {
	cycles                                   int
	sweep, assist, dedicated, idle, markTerm float64
}

// compulsory is GC work the scheduler could not have avoided: everything but
// idle mark, which runs only on processors nothing else wanted. runtime/metrics
// documents this same subtraction for /cpu/classes/gc/mark/idle.
func (s stats) compulsory() float64 { return s.sweep + s.assist + s.dedicated + s.markTerm }

func (s stats) total() float64 { return s.compulsory() + s.idle }

// gcLine matches the CPU group of a gctrace record. The full line reads:
//
//	gc 41 @0.414s 1%: 0.028+2.6+0.012 ms clock, 0.90+0.041/18/23+0.41 ms cpu, 245->248->139 MB, ...
//
// and the group before " ms cpu" is
// sweepTerm + markAssist/markDedicated/markIdle + markTerm, in milliseconds.
// Anchoring on "ms clock, " and " ms cpu" keeps this indifferent to the
// fields around it, which have changed across Go releases.
var gcLine = regexp.MustCompile(`^gc \d+ .*ms clock, ([0-9.+/]+) ms cpu,`)

func parse(trace string) stats {
	var s stats
	for _, line := range strings.Split(trace, "\n") {
		m := gcLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		plus := strings.Split(m[1], "+")
		if len(plus) != 3 {
			continue
		}
		marks := strings.Split(plus[1], "/")
		if len(marks) != 3 {
			continue
		}
		s.cycles++
		s.sweep += ms(plus[0])
		s.assist += ms(marks[0])
		s.dedicated += ms(marks[1])
		s.idle += ms(marks[2])
		s.markTerm += ms(plus[2])
	}
	return s
}

// ms converts one millisecond field of a gctrace record to seconds. A field
// that will not parse contributes zero rather than aborting the run: losing
// one cycle out of hundreds cannot flip a budget, but refusing to report at
// all over a format change would take the gate offline silently.
func ms(f string) float64 {
	v, err := strconv.ParseFloat(f, 64)
	if err != nil {
		return 0
	}
	return v / 1000
}
