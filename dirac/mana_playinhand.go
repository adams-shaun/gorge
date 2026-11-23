package dirac

// playInHand reports whether the searched cast's own card is still in p's
// hand: gorge's scriptStateKey marks it beside the hand's size, so a loot that
// discards that card and one that discards another (same hand size) are two
// payer states (CR 601.2a: a card is cast from where it is). Every search
// that mirrors the key reads it here. A non-cast play marks nothing.
func (t *State) playInHand(p uint8, a Action) bool {
	if a.Kind() != AkCast {
		return false
	}
	pl := &t.pl[SeatIx(p)]
	src := Obj(a.Source())
	for _, o := range pl.hand[:pl.handN] {
		if o == src {
			return true
		}
	}
	return false
}
