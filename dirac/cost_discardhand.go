package dirac

// payDiscardHandCost pays kop.Cost.DiscardHand: every card in the payer's
// hand, in hand order, as part of an activated ability's cost (gorge
// pay.DiscardCandidates' whole-hand spec and pay.PayDiscardCost's one
// discard action). No choice is asked and an empty hand pays with nothing
// (CR 118.3, 602.2b), so there is no offer gate.
func (s *State) payDiscardHandCost(k *prep, p uint8) {
	pl := &s.pl[p]
	n := int(pl.handN)
	var picks [MaxHand]Obj
	copy(picks[:n], pl.hand[:n])
	for i := 0; i < n; i++ {
		s.discardCost(k, picks[i])
	}
}
