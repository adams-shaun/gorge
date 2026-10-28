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

import (
	"fmt"
	"testing"
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
			if out.misses > 0 {
				t.Logf("production bot pick missed a v2 candidate on %d substep(s); the heads still agree", out.misses)
			}
		})
	}
}
