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
	s.discardHandMatching(k, p, f)
}

// discardWholeHand discards every card in p's hand (a token copy waits in the
// hand for the state-based actions) without an ask: one discard action, hence
// one zone batch (gorge flushDiscards). It lives here, not in exec, so the
// interpreter keeps no defer.
func (s *State) discardWholeHand(k *prep, p uint8) {
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // one discard action (gorge flushDiscards)
	}
	pl := &s.pl[p]
	for i := uint8(0); i < pl.handN; {
		if s.objs[pl.hand[i]].of&ofCopy != 0 || s.park(k, pl.hand[i], parkDiscard, 0) {
			i++
			continue
		}
		s.discardObj(k, pl.hand[i])
	}
}

// discardHandMatching discards p's hand cards matching f, without an ask. The
// whole op is one discard action, hence one zone batch (gorge flushDiscards),
// so a ChangesZoneAll "from anywhere" line queues once for the hand, not per
// card.
func (s *State) discardHandMatching(k *prep, p uint8, f kop.FilterID) {
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // one discard action (gorge flushDiscards)
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
