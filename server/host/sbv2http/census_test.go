package sbv2http

// TestSBV2NativeOnlyCensus ratchets the decisions the v2 wire cannot pose:
// over the same 20-seed legacy-deck corpus as TestSBV2ParityLegacyDecks, it
// counts the requests served with x_gorge_human_v1.native_only, keyed by the
// native decision kind in the accompanying x_gorge_view_v1 render plane.
//
// testdata/native_only.tsv is the checked-in baseline. It may only shrink: a
// kind that gains decisions fails here, and a kind whose count dropped (or
// disappeared) also fails until the baseline is updated, so the table cannot
// rot above the measured gap. Regenerate it with SBV2_NATIVE_ONLY_REGEN=1
// after a change that legitimately moves the count.

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const nativeOnlyBaselinePath = "testdata/native_only.tsv"

func TestSBV2NativeOnlyCensus(t *testing.T) {
	res := parityCorpus(t)
	if len(res.errs) > 0 {
		for _, e := range res.errs {
			t.Errorf("%s", e)
		}
		t.FailNow()
	}
	if len(res.outcomes) != len(paritySeeds) {
		t.Fatalf("corpus produced %d seed outcomes, want %d", len(res.outcomes), len(paritySeeds))
	}

	measured := map[string]int{}
	for _, out := range res.outcomes {
		if len(out.errs) > 0 {
			for _, e := range out.errs {
				t.Errorf("seed %d: %s", out.seed, e)
			}
			continue
		}
		for kind, n := range out.census {
			measured[kind] += n
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	// The census only means something if the corpus actually played; an empty
	// census (every decision mapped) is the target, so the check is that each
	// seed ran v2 decisions, not that it served native_only ones.
	if err := censusRunProblem(res); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, n := range measured {
		total += n
	}
	// Card-name asks past the 4096-candidate window are answered over the v2
	// wire's choose_name action; the corpus holds three (seeds 2004, 2011,
	// 2016), so none being sent means the action path went unexercised.
	nameSends := 0
	for _, out := range res.outcomes {
		nameSends += out.nameSends
	}
	if nameSends == 0 {
		t.Fatal("precondition: the corpus answered no windowed card-name ask through x_gorge_human_v1.choose_name")
	}
	t.Logf("nameSends=%d", nameSends)

	got := []byte(renderNativeOnlyCensus(measured))
	if os.Getenv("SBV2_NATIVE_ONLY_REGEN") == "1" {
		// Refuse to bless growth. A kind that gained decisions is a regression
		// the ratchet exists to catch; only a shrink (or a kind that
		// disappeared) may be written without review.
		if prior, err := os.ReadFile(nativeOnlyBaselinePath); err == nil {
			before := parseNativeOnlyCensus(t, prior)
			for kind, n := range measured {
				if n > before[kind] {
					t.Fatalf("refusing to regenerate %s: %s grew from %d to %d; the baseline may only shrink",
						nativeOnlyBaselinePath, kind, before[kind], n)
				}
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("reading %s before regen: %v", nativeOnlyBaselinePath, err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(nativeOnlyBaselinePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("regenerated %s (%d kinds, %d decisions)", nativeOnlyBaselinePath, len(measured), total)
		return
	}

	want, err := os.ReadFile(nativeOnlyBaselinePath)
	if err != nil {
		t.Fatalf("reading %s: %v (generate with SBV2_NATIVE_ONLY_REGEN=1 go test -run TestSBV2NativeOnlyCensus ./server/host/sbv2http/)",
			nativeOnlyBaselinePath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("native-only census moved. testdata/native_only.tsv is a ratchet: it may only shrink, and a stale row must be removed.\n--- measured ---\n%s--- baseline ---\n%s",
			got, want)
	}
}

// censusRunProblem reports a corpus run too thin to ratchet against: a seed
// that never reached a mapped v2 decision proves nothing about the census.
func censusRunProblem(res parityCorpusResult) error {
	for _, out := range res.outcomes {
		if out.picks == 0 {
			return fmt.Errorf("precondition: seed %d answered no mapped v2 decision (picks 0)", out.seed)
		}
	}
	return nil
}

func TestCensusRunProblem(t *testing.T) {
	if err := censusRunProblem(parityCorpusResult{outcomes: []seedOutcome{{seed: 1, picks: 1}}}); err != nil {
		t.Fatalf("an empty census with picks must pass: %v", err)
	}
	if err := censusRunProblem(parityCorpusResult{outcomes: []seedOutcome{{seed: 1, picks: 1}, {seed: 2, picks: 0}}}); err == nil {
		t.Fatal("an outcome with no picks must be a problem")
	}
}

// renderNativeOnlyCensus writes the sorted, stable TSV baseline.
func renderNativeOnlyCensus(counts map[string]int) string {
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	var b strings.Builder
	for _, kind := range kinds {
		fmt.Fprintf(&b, "%s\t%d\n", kind, counts[kind])
	}
	return b.String()
}

// parseNativeOnlyCensus reads a baseline so regen can refuse to bless growth.
func parseNativeOnlyCensus(t *testing.T, raw []byte) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			t.Fatalf("%s: malformed line %q, want kind<TAB>count", nativeOnlyBaselinePath, line)
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 0 {
			t.Fatalf("%s: bad count in %q: %v", nativeOnlyBaselinePath, line, err)
		}
		out[fields[0]] = n
	}
	return out
}
