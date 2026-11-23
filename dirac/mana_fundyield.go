package dirac

// fundYieldsNothing reports whether the funding source at sl is a dynamic-amount
// mana source that makes no mana now (Lotus Blossom with no petal counters).
// gorge's prefix search never taps such a covered source to fund a gap's fee
// (scriptSearch.activations: a covered source windowSourceYields refuses is no
// branch, so it spends none of the 164-node budget), and the kernel's funding
// pass must skip it the same way: tapping it costs a node and a colour ask per
// branch, which starved the search of the granted double mana that paid
// Fangs of Kalonia's overload cost (tail-74-c000 pair 126, decision 200).
func (s *State) fundYieldsNothing(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	return k.gen.flags[c]&gfManaVar != 0 && s.varManaEmpty(k, sl, c)
}
