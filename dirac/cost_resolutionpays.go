package dirac

// paySacSelf pays kop.PaySacSelf: the resolving ability's source, still
// on the battlefield as the same object, is sacrificed (gorge's triggered
// cost window settles Sac<1/CARDNAME> unasked: its only candidate).
//
//go:noinline
func (s *State) paySacSelf(k *prep) {
	if sl := s.resSrcSlot(k); sl != NoSlot {
		s.fireSac(k, sl)
		s.leaveParked(k, sl)
	}
}

// payDraw pays kop.PayDraw: the payer draws n cards (gorge cumulative.go's
// Draw-bearing triggered cost: drawCostCard per count; an empty library is
// a draw from it, CR 121.4).
//
//go:noinline
func (s *State) payDraw(p uint8, n int) {
	for i := 0; i < n && !s.over; i++ {
		s.draw(p)
	}
}
