package dirac

// --- Duration$ UntilUntaps on an Animate ---
//
// Hedge Whisperer's "target land you control becomes a 5/5 green Plant Boar
// creature with haste for as long as Hedge Whisperer remains tapped" (CR
// 611.2b). gorge registers every half of the animation with UntilUntapOf the
// activating host (registerAnimateEffects): nothing when the host is not a
// tapped permanent as it resolves, and expireUntilUntaps drops it at the
// host's first untap or departure, so a retap never revives it. The kernel's
// record is a WhileHost override (filter_combatwords.go: the host's Obj and
// ver in the record's P/T fields, the departure in hostLeft) marked
// kop.Override.WhileTapped; the untap sites call untapEnds.

// untapEnds is what the untap of the permanent o ends: a Braided Net lock and
// the Panharmonicon grants it hosts (netDrop), and the WhileTapped override
// records it hosts.
func (s *State) untapEnds(k *prep, o Obj) {
	if s.g.netAny() {
		s.netDrop(o) // act_lockpermanent.go
	}
	if k.gen != nil && k.gen.anyWhileTapped && s.fxM.any() {
		s.whileTappedEnd(k, o)
	}
}

// whileTappedEnd drops the WhileTapped override records whose host is o.
//
//go:noinline
func (s *State) whileTappedEnd(k *prep, o Obj) {
	s.dropFx(func(i int) bool {
		r := &s.g.fx[i]
		return r.kind == fxOverride && k.t.Overrides[r.r.idx-1].WhileTapped && Obj(r.r.p) == o
	})
	s.refreshTy(k)
}
