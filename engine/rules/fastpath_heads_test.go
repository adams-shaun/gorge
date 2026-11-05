package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFastPathsInProductionModeKeepTheHeads plays the acceptance games with
// the SBA quiet skip/prefilters (sbaquiet.go, sba_prefilter.go) and the plain
// mana fast path (mana_plain.go) in their PRODUCTION mode -- the rules test
// binary otherwise runs both in verify mode, which takes the general path
// and only compares -- and requires the pinned chain heads
// (engine/rules/testdata/heads/). Not parallel: it flips package-level verify
// flags, which no parallel test may observe. Its seat-count games are
// independent parallel subtests, with the flags held for the whole group by
// t.Cleanup (which runs after the subtests, unlike a defer).
func TestFastPathsInProductionModeKeepTheHeads(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	prevSBA, prevMana := sbaQuietVerify, manaPlainVerify
	sbaQuietVerify, manaPlainVerify = false, false
	t.Cleanup(func() { sbaQuietVerify, manaPlainVerify = prevSBA, prevMana })
	for _, seats := range []int{2, 4} {
		seats := seats
		t.Run(fmt.Sprintf("%dseats", seats), func(t *testing.T) {
			t.Parallel()
			if got, want := acceptanceHead(t, reg, seats), pinnedHead(t, seats); got != want {
				t.Errorf("%d seats: production-mode chain head %s, golden %s", seats, got, want)
			}
		})
	}
}
