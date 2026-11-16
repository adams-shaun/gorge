package rules

// The offer's manual-payment witness: what the planner may not plan, the
// payment window can still pay.

import "github.com/adams-shaun/gorge/engine/state"

// manualWitnessPlain is autoPayFastPlain widened by the part class the
// payment window settles on its own: the payer's discarding the source
// itself (a Channel from hand -- the discard is a cost the window's own
// ask completes). autoPayFastPlain stays the planner-exact shape: the
// planner keeps a sacrificed source out of every consuming alternative but
// reads no discard at all, so the fast path cannot widen it.
func manualWitnessPlain(composed *Cost, id state.ObjID) bool {
	plain := Cost{Colored: composed.Colored, Generic: composed.Generic, Tap: composed.Tap}
	if autoPaySelfParts(composed.Sac, id) {
		plain.Sac = composed.Sac
	}
	if autoPaySelfParts(composed.Exile, id) {
		plain.Exile = composed.Exile
	}
	if autoPaySelfParts(composed.Discard, id) {
		plain.Discard = composed.Discard
	}
	return composed.Equal(&plain)
}
