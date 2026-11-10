package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// delayedSelfStale reports whether the resolving entry is a delayed
// trigger (a latent ability a card's OpDelayedProgram registers) whose
// source card has re-entered the battlefield since the trigger fired. gorge's
// delayed trigger names its host card, not the incarnation that registered
// it: Fortune, Loyal Steed's second end-of-combat trigger still exiles (and
// returns) the Fortune its first trigger blinked, though CR 400.7 calls that
// a new object. moveObj's RefSelf version gate (a stale ETB or activated
// ability finds nothing) therefore passes such an entry. A tracked promise
// (dpTrackAmt) keeps the gate: its resolve already drops a new object.
//
//go:noinline
func (s *State) delayedSelfStale(k *prep, e *stackEntry) bool {
	if e.kind != seTrigger || e.trigAmt == dpTrackAmt || int(e.card) >= len(k.t.Cards) {
		return false
	}
	abs := k.t.Cards[e.card].Abilities
	for i := range abs {
		for _, in := range abs[i].Prog {
			if in.Op == kop.OpDelayedProgram && uint8(in.C) == e.ab {
				return true
			}
		}
	}
	return false
}
