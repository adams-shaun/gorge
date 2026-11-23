package dirac

// tapsFilterFallbackOK gates the Taps-deferred fallback search beside a
// filter (script_gen.go): gorge's one prefix search over the census gaps
// (filter and Taps-deferred sources together) is the offer's only judge, and
// the fallback stands in for a phased run the kernel's filter route cannot
// replay where the deferred taps resolve at the tap (Rikala's gated extra
// beside Trapjaw Kelpie: a mana-ability trigger queues nothing). A tap that
// hurts its payer queues a stack trigger, and a sorcery-speed play must wait
// for each (CR 307.1, 602.5d): gorge's search then also branches on the pass
// that lets it resolve, spends its node budget before the shorter prefix, and
// offers nothing -- as the filter route's own exhausted run says
// (Overflowing Basin beside five Islands under Scald, Fiery Mantle at
// three life). The fallback's shorter prefix would offer what gorge does not.
func (s *State) tapsFilterFallbackOK(k *prep, a Action, p uint8, tapsDef slotMask) bool {
	if !s.playNeedsEmptyStack(k, a) {
		return true
	}
	for m := tapsDef; m.any(); m = m.rest() {
		if s.tapsCensusPain(k, p, m.first()) > 0 {
			return false
		}
	}
	return true
}

// tapsFilterFallbackScript reports the Taps-deferred fallback search beside a
// filter (script_gen.go) finding a script for the play: deferred taps are
// among the gaps, tapsFilterFallbackOK admits the fallback, and the dead-end
// search over those gaps is not a dead end.
func (s *State) tapsFilterFallbackScript(k *prep, a Action, p uint8, restr, anyR, tapsDef, anyDiff slotMask) bool {
	return tapsDef.any() && s.tapsFilterFallbackOK(k, a, p, tapsDef) &&
		!s.scriptDeadEndAny(k, a, p, restr, anyR, tapsDef, s.scriptCtr(k, p), anyDiff)
}
