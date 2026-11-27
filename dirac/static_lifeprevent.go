package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// lifePreventCompetes reports that p's gain, swallowed by a printed
// "that player gains no life instead" replacement (a StatCantGainLife with
// Static.A 1, lower_lifeprevent.go), meets a gain-doubling or plus-N
// replacement of p. gorge keeps that line out of its CantGainLife restriction
// (replacement_life.go lifeGainForbidden), so it competes in the CR 616.1
// order ask beside them, and the order changes what a later replacement
// sees; the kernel's static poses no such ask, so the gain site stalls. A gain
// a real restriction (a CantGainLife static with A 0, an Effect's lock)
// forbids is gorge's before any replacement and is no competition.
func (s *State) lifePreventCompetes(k *prep, p uint8) bool {
	if k.gen.anyRepl&(rbDblLife|rbPlusLife) == 0 || s.pl[p].mark&pmNoGain != 0 {
		return false
	}
	repl, restr := false, false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatCantGainLife &&
			(st.a.Static.StFlags&kop.StBanOpp == 0 || st.ctrl != p) && s.statCond(k, st) {
			if st.a.Static.A == 1 {
				repl = true
			} else {
				restr = true
			}
		}
		return true
	})
	if !repl || restr {
		return false
	}
	_, c, _, ok := s.lifeCands(k, p)
	return !ok || c > 0
}
