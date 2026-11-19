package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// A new replacement event bit must not look like a conditional target-entry
// reason to the ask-free resolution predicate (or checkpoint a vanilla spell).
func TestLoseManaGateDoesNotAliasTargetEntry(t *testing.T) {
	if cards.ReplLoseMana == 0 || cards.ReplLoseMana >= cards.ReplEventCount {
		t.Fatal("precondition: LoseMana is not a replacement event")
	}
	// Check the full gate mask, not only LoseMana: future vocabulary additions
	// must move the conditional bits too (the cache state widened to uint64
	// when ReplEventCount reached 31).
	gates := uint64((cards.ReplEventMask(1)<<cards.ReplEventCount)-1) << mayAskGateShift
	loseMana := uint64(1) << (mayAskGateShift + cards.ReplLoseMana)
	if gates&loseMana == 0 || gates == 0 {
		t.Fatal("precondition: LoseMana is not present in the packed gate mask")
	}
	for _, reason := range []uint64{uint64(cards.MayAskCondTargetEntry), uint64(cards.MayAskCondParentSub)} {
		packed := reason << mayAskCondShift
		if packed == 0 {
			t.Fatalf("conditional reason %d does not fit the cache", reason)
		}
		if gates&packed != 0 {
			t.Fatalf("replacement gates %#x overlap conditional reason %d at shift %d", gates, reason, mayAskCondShift)
		}
	}
}
