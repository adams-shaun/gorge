package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNoDefinedDiscard (TestFaadiyahSeerMutation) makes an OptDefined
// discard take nothing: gorge still discards the defined card, so the pair
// diverges at the hand and graveyard projections.
var DebugNoDefinedDiscard bool

// discardDefinedAll runs an OpDiscard with kop.OptDefined (gorge effDiscard's
// Mode$ Defined arm, engine/effects/cardflow.go: the cards DefinedCards$ names
// that are still in the discarder's hand move, no ask, the amount never read):
// every hand card matching the filter is discarded in hand order. The core's
// (interp.go) and gen's (interp_gen.go) arms share this helper so both sides
// take exactly the same shape.
func (s *State) discardDefinedAll(k *prep, p uint8, f kop.FilterID) {
	if DebugNoDefinedDiscard {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); {
		o := pl.hand[i]
		if s.objs[o].of&ofCopy == 0 && s.discardMatch(k, f, o) && !s.park(k, o, parkDiscard, 0) {
			s.discardObj(k, o)
			continue
		}
		i++
	}
}
