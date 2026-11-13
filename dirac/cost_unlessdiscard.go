package dirac

// the unless-cost discard selection methods gorge added in
// d66db41d8 (pay/unlessflow.go). A Random part draws the engine's RNG once
// per card over the still-eligible hand cards (hand order, each pick removed
// in place), and the picks are discarded in pick order. A Hand part is the
// whole hand in hand order; an empty hand pays with nothing.

// payDiscardRandom pays Discard<N/Random> for p.
func (s *State) payDiscardRandom(k *prep, p uint8, n int) {
	pl := &s.pl[p]
	var eligible [MaxHand]Obj
	m := int(pl.handN)
	copy(eligible[:m], pl.hand[:m])
	var picks [MaxHand]Obj
	np := 0
	for i := 0; i < n; i++ {
		j := s.rng.IntN(m)
		picks[np] = eligible[j]
		np++
		copy(eligible[j:], eligible[j+1:m])
		m--
	}
	for i := 0; i < np; i++ {
		s.discardUnlessPay(k, picks[i])
	}
}

// payDiscardHand pays Discard<N/Hand> for p: the whole hand, in hand order.
func (s *State) payDiscardHand(k *prep, p uint8) {
	pl := &s.pl[p]
	n := int(pl.handN)
	var picks [MaxHand]Obj
	copy(picks[:n], pl.hand[:n])
	for i := 0; i < n; i++ {
		s.discardUnlessPay(k, picks[i])
	}
}

// FeatStackKicked reports whether stack entry i was kicked (gorge's
// StackView.Kicks > 0, entity slot entStackKicked). Gorge sets FlagKicked
// when the cast is paid (rules/cast_commit.go payCast), so a spell still
// being built shows no kick yet; and kernel sfKicked is also the offspring
// and buyback flag, which gorge reports as FlagOffspringPaid / FlagBuyback,
// not Kicks (an entwined cast, which gorge flags with nothing, is no kick
// either: kw_entwine.go).
func (s *State) FeatStackKicked(i int) bool {
	if i < 0 || i >= int(s.sp) || s.stack[i].flags&sfKicked == 0 {
		return false
	}
	if s.ctl == ctlBuild && s.bld().e.obj == s.stack[i].obj {
		return false
	}
	c := s.stack[i].card
	return int(c) >= len(s.k().t.Cards) || !s.k().t.Cards[c].Offspring && !s.k().t.Cards[c].Buyback && !s.k().t.Cards[c].Entwine
}
