package pay

import "github.com/adams-shaun/gorge/engine/state"

// anyamount.go is the voluntary "pay any amount of mana" additional cost
// shape (Chorus of the Conclave, CR 601.2b/118.8): a free integer chosen at
// cast time, bounded only by what the caster can still pay.

// AnyAmountCap bounds the amount the ask can offer. It exists so a
// degenerate pool cannot build an unbounded option list; no real pool comes
// near it.
const AnyAmountCap int32 = 256

// MaxAnyAmount returns the largest n in [0, AnyAmountCap] for which ok(n)
// holds. ok must be monotone (paying more is never easier), which every
// cost-payability check is, so the search is a bisection over the cap.
func MaxAnyAmount(ok func(n int32) bool) int32 {
	lo, hi := int32(0), AnyAmountCap
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		if ok(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// AppendAnyAmountGrant appends the entry rider an amount paid earns to the
// cast's AddsCounters grants: n units of one +1/+1 counter each, in the shape
// state.ManaAddsCounterGrant takes, so the battlefield-entry counter plan
// places them with the creature's other enters-with counters and a replay
// reconstructs the same grant from the cast's CastInfo payload. The filter is
// the bare card spec: the grant belongs to the spell that paid, whatever it
// is when it enters. A zero amount appends nothing.
func AppendAnyAmountGrant(grants []state.ManaAddsCounterGrant, n int32) []state.ManaAddsCounterGrant {
	if n <= 0 {
		return grants
	}
	return append(grants, state.ManaAddsCounterGrant{Filter: "Card", Kind: "P1P1", Amount: "1", Count: n})
}
