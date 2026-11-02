package dirac

import (
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestTransitionAllocatesNothing plays BenchmarkTransitionFDN's loop: a transition
// that allocates grows without bound during training, where GOGC is off.
func TestTransitionAllocatesNothing(t *testing.T) {
	games := fdnBenchTables(t, 64)
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := 0
	cfg := Config{Mulligans: 2}
	s.Init(games[0].tbl, games[0].da, games[0].db, 1, cfg)
	allocs := testing.AllocsPerRun(100_000, func() {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			gm := &games[g%len(games)]
			s.Init(gm.tbl, gm.da, gm.db, uint64(g)+1, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	})
	if allocs != 0 {
		t.Errorf("%v allocs per transition", allocs)
	}
}

// inliningReport is the compiler's -m=2 output for this package.
var inliningReport = sync.OnceValues(func() (string, error) {
	out, err := exec.Command("go", "build", "-gcflags=-m=2", ".").CombinedOutput()
	return string(out), err
})

func compilerReport(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	out, err := inliningReport()
	if err != nil {
		t.Fatalf("go build -gcflags=-m=2: %v\n%s", err, out)
	}
	return out
}

// TestHotAccessorsInline: a hot accessor that stops inlining costs a call on every
// read of a permanent.
func TestHotAccessorsInline(t *testing.T) {
	out := compilerReport(t)
	const hot = `\(\*State\)\.(typesAt|basePT)\b|\(\*?slotMask\)\.|slotMask\.[a-zA-Z]+\b|slotBit\b`
	for _, line := range regexp.MustCompile(`.*cannot inline (`+hot+`).*`).FindAllString(out, -1) {
		t.Errorf("stopped inlining: %s", line)
	}
	if n := len(regexp.MustCompile(`can inline \(\*State\)\.(typesAt|basePT)\b`).FindAllString(out, -1)); n < 2 {
		t.Errorf("found %d of (*State).typesAt and (*State).basePT in the -m=2 output", n)
	}
}

// TestNoKernelLocalEscapes: a local moved to the heap allocates on every visit.
func TestNoKernelLocalEscapes(t *testing.T) {
	for _, line := range strings.Split(compilerReport(t), "\n") {
		if regexp.MustCompile(`^[^ ]+\.go:[0-9]+:[0-9]+: moved to heap: `).MatchString(line) {
			t.Error(line)
		}
	}
}
