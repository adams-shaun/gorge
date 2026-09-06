package main

import (
	"math"
	"testing"
)

// Two verbatim records from a GODEBUG=gctrace=1 run of ./host on go1.26.3.
// Kept exactly as the runtime emitted them: the parser's whole job is to
// survive this format, so paraphrasing them would test nothing.
const (
	traceA = "gc 41 @0.414s 1%: 0.028+2.6+0.012 ms clock, 0.90+0.041/18/23+0.41 ms cpu, 245->248->139 MB, 250 MB goal, 0 MB stacks, 0 MB globals, 32 P"
	traceB = "gc 56 @1.477s 1%: 0.036+9.2+0.012 ms clock, 1.1+0.25/72/167+0.39 ms cpu, 972->979->552 MB, 1006 MB goal, 0 MB stacks, 0 MB globals, 32 P"
)

func close(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.9f, want %.9f", what, got, want)
	}
}

// The CPU group is sweepTerm + assist/dedicated/idle + markTerm, and each
// class lands in its own bucket. Reading the group positionally is the one
// thing that can silently mis-attribute idle mark as compulsory work, which
// is exactly the distinction the budget turns on.
func TestParseSumsEachGCClassSeparately(t *testing.T) {
	s := parse(traceA + "\n" + traceB + "\n")

	if s.cycles != 2 {
		t.Fatalf("cycles = %d, want 2", s.cycles)
	}
	close(t, "sweep", s.sweep, (0.90+1.1)/1000)
	close(t, "assist", s.assist, (0.041+0.25)/1000)
	close(t, "dedicated", s.dedicated, (18+72)/1000.0)
	close(t, "idle", s.idle, (23+167)/1000.0)
	close(t, "markTerm", s.markTerm, (0.41+0.39)/1000)
}

// total must include idle mark and compulsory must exclude it. If these two
// ever agree, the -compulsory-only flag is a no-op and the default budget has
// quietly stopped counting the largest class in the trace.
func TestTotalCountsIdleMarkAndCompulsoryExcludesIt(t *testing.T) {
	s := parse(traceA + "\n" + traceB + "\n")

	wantCompulsory := (0.90 + 1.1 + 0.041 + 0.25 + 18 + 72 + 0.41 + 0.39) / 1000
	close(t, "compulsory()", s.compulsory(), wantCompulsory)
	close(t, "total()", s.total(), wantCompulsory+(23+167)/1000.0)

	if s.total() <= s.compulsory() {
		t.Fatal("total() must exceed compulsory() when the trace contains idle-mark CPU")
	}
	close(t, "total()-compulsory()", s.total()-s.compulsory(), s.idle)
}

// A trace is interleaved with the suite's own output and with the runtime's
// other GODEBUG lines. Counting any of them would inflate the numerator.
func TestParseIgnoresEverythingThatIsNotAGCRecord(t *testing.T) {
	noise := "" +
		"scvg: inuse: 1, idle: 0, sys: 1, released: 0, consumed: 1 (MB)\n" +
		"=== RUN   TestSomething\n" +
		"--- PASS: TestSomething (0.01s)\n" +
		"PASS\n" +
		"gc 1 @0.001s 0%: no cpu group here\n" +
		"\n"
	s := parse(noise)
	if s.cycles != 0 {
		t.Fatalf("cycles = %d, want 0 — a non-record was counted as a GC cycle", s.cycles)
	}
	if s.total() != 0 {
		t.Fatalf("total() = %v, want 0", s.total())
	}
}

// A forced collection (runtime.GC, or the memory limit) is a real cycle and
// its CPU is real CPU. The trailing "(forced)" must not exclude it.
func TestParseCountsAForcedCollection(t *testing.T) {
	forced := "gc 7 @0.100s 0%: 0.010+0.20+0.003 ms clock, 0.32+0/0.10/0.15+0.11 ms cpu, 4->4->2 MB, 5 MB goal, 0 MB stacks, 0 MB globals, 32 P (forced)"
	s := parse(forced)
	if s.cycles != 1 {
		t.Fatalf("cycles = %d, want 1 — a forced collection was skipped", s.cycles)
	}
	close(t, "dedicated", s.dedicated, 0.10/1000)
	close(t, "idle", s.idle, 0.15/1000)
}

// A future Go release can change the fields around the CPU group. Dropping a
// record we cannot read loses a fraction of a percent; aborting, or silently
// reporting zero cycles, would take the gate offline. gate() rejects a
// zero-cycle run for that reason, so the parser only has to not panic.
func TestParseSurvivesAMalformedRecordWithoutLosingTheGoodOnes(t *testing.T) {
	s := parse("gc 3 @0.0s 0%: 1+2+3 ms clock, garbage/here ms cpu, 1->1->1 MB\n" + traceA + "\n")
	if s.cycles != 1 {
		t.Fatalf("cycles = %d, want 1 — the malformed record should be dropped and the good one kept", s.cycles)
	}
	close(t, "dedicated", s.dedicated, 18.0/1000)
}

// ms is the only unit conversion in the tool; a factor-of-1000 slip here
// moves every reported figure without failing anything else.
func TestMsConvertsMillisecondsToSecondsAndZeroesWhatItCannotRead(t *testing.T) {
	close(t, "ms(\"250\")", ms("250"), 0.25)
	close(t, "ms(\"0.041\")", ms("0.041"), 0.000041)
	close(t, "ms(\"\")", ms(""), 0)
	close(t, "ms(\"nonsense\")", ms("nonsense"), 0)
}
