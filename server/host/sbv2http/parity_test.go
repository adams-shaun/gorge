package sbv2http

// TestSBV2ParityLegacyDecks proves the opt-in /api/sbv2 wire is lossless for
// play on the repo's 2-seat Legacy deck pairs: for each fixed seed, a native
// table whose two seats are the production default bots and a v2 table at the
// same seed whose seat 0 is a scripted HTTP client playing the production
// bot's own picks must finish with the same event-log chain head.
//
// The two tables share everything parity depends on: the same table seed,
// deck pair, mulligan setting and payment posture. Seat 0's client bot is
// built with the exact bots.New options the native table's seat 0 bot gets
// (match seed ^ 1, auto-payment, no deadline), so a head mismatch names the
// transport or the mapping, not a bot difference.
//
// The client's native fallbacks are ratcheted in testdata/parity_misses.tsv,
// per seed. A pick with no v2 candidate is answered over the native routes, so
// a mapping or pose regression that empties the candidate list would otherwise
// still reach the native head and pass. Both a rise and a fall fail, so the
// table cannot rot; regenerate after a change that legitimately moves it with
// SBV2_PARITY_MISSES_REGEN=1. The only measured cause is auto-pay hiding
// priority `activate` candidates (see the parity_corpus driver): the
// production bot can still pick an explicit mana activation.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	parityMissBaselinePath = "testdata/parity_misses.tsv"

	// paritySendFloorPercent is the minimum share of mapped decisions the
	// scripted client must answer over /send. The only legitimate misses are
	// auto-pay-hidden priority activations, well under one per decision; a
	// mapping regression that turned picks into native fallbacks would sink
	// this ratio even on a seed whose miss count is still within its ratchet.
	paritySendFloorPercent = 90
)

func TestSBV2ParityLegacyDecks(t *testing.T) {
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

	regen := os.Getenv("SBV2_PARITY_MISSES_REGEN") == "1"
	var baseline map[uint64]int
	if !regen {
		baseline = readParityMissBaseline(t)
	}

	totalPicks, totalSends, totalMisses := 0, 0, 0
	for _, out := range res.outcomes {
		out := out
		t.Run(fmt.Sprintf("seed-%d", out.seed), func(t *testing.T) {
			if len(out.errs) > 0 {
				for _, e := range out.errs {
					t.Errorf("%s", e)
				}
				t.FailNow()
			}
			// Preconditions: both games ran and produced a nontrivial log,
			// so an empty/empty "match" cannot pass as parity.
			if out.nativeHead == "" || out.wireHead == "" {
				t.Fatalf("empty chain head: native %q, wire %q", out.nativeHead, out.wireHead)
			}
			if out.nativeEvents <= 0 || out.wireEvents <= 0 {
				t.Fatalf("empty event log: native %d events, wire %d events", out.nativeEvents, out.wireEvents)
			}
			if out.nativeHead != out.wireHead {
				t.Fatalf("chain heads differ: native %s (%d events), wire %s (%d events)",
					out.nativeHead, out.nativeEvents, out.wireHead, out.wireEvents)
			}
			// The client must actually have driven the v2 wire, not answered
			// the whole game through the native fallback.
			if out.picks == 0 {
				t.Fatalf("precondition: the v2 client was offered no mapped decisions")
			}
			if !regen {
				want, ok := baseline[out.seed]
				if !ok {
					t.Fatalf("seed %d missing from %s (stale or incomplete baseline; regenerate with SBV2_PARITY_MISSES_REGEN=1)", out.seed, parityMissBaselinePath)
				}
				if out.misses != want {
					t.Fatalf("native fallbacks moved: got %d, baseline %s has %d. testdata/parity_misses.tsv is a ratchet: a rise is a mapping regression, a fall is a stale row; regenerate with SBV2_PARITY_MISSES_REGEN=1 after a legitimate change",
						out.misses, parityMissBaselinePath, want)
				}
			}
		})
		totalPicks += out.picks
		totalSends += out.sends
		totalMisses += out.misses
	}

	if totalPicks == 0 {
		t.Fatal("precondition: the v2 clients were offered no mapped decisions")
	}
	// A 409 retry counts its request twice in totalPicks; sends+misses counts
	// each mapped decision once.
	mapped := totalSends + totalMisses
	t.Logf("mapped decisions: %d over /send, %d native fallbacks", totalSends, totalMisses)
	if mapped == 0 {
		t.Fatal("precondition: the v2 clients answered no mapped decisions")
	}
	if totalSends*100 < mapped*paritySendFloorPercent {
		t.Fatalf("only %d of %d mapped decisions (%d%%) went over /send; the floor is %d%%",
			totalSends, mapped, totalSends*100/mapped, paritySendFloorPercent)
	}

	if regen {
		writeParityMissBaseline(t, res.outcomes)
		return
	}
	for seed := range baseline {
		found := false
		for _, out := range res.outcomes {
			if out.seed == seed {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("stale seed %d in %s: the corpus no longer runs it", seed, parityMissBaselinePath)
		}
	}
}

// readParityMissBaseline reads the per-seed native-fallback ratchet.
func readParityMissBaseline(t *testing.T) map[uint64]int {
	t.Helper()
	raw, err := os.ReadFile(parityMissBaselinePath)
	if err != nil {
		t.Fatalf("reading %s: %v (generate with SBV2_PARITY_MISSES_REGEN=1 go test -run TestSBV2ParityLegacyDecks ./server/host/sbv2http/)",
			parityMissBaselinePath, err)
	}
	out := map[uint64]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			t.Fatalf("%s: malformed line %q, want seed<TAB>misses", parityMissBaselinePath, line)
		}
		seed, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			t.Fatalf("%s: bad seed in %q: %v", parityMissBaselinePath, line, err)
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 0 {
			t.Fatalf("%s: bad miss count in %q: %v", parityMissBaselinePath, line, err)
		}
		if _, dup := out[seed]; dup {
			t.Fatalf("%s: duplicate seed %d", parityMissBaselinePath, seed)
		}
		out[seed] = n
	}
	return out
}

// writeParityMissBaseline writes the sorted, stable per-seed TSV baseline.
func writeParityMissBaseline(t *testing.T, outcomes []seedOutcome) {
	t.Helper()
	var b strings.Builder
	for _, out := range outcomes {
		fmt.Fprintf(&b, "%d\t%d\n", out.seed, out.misses)
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parityMissBaselinePath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("regenerated %s (%d seeds)", parityMissBaselinePath, len(outcomes))
}
