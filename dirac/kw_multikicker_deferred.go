package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kickedPotentialSansDeferred prices a sorcery-speed kicked cast's potential
// arm without the Taps-deferred sources (Scald beside an Island, CR 603.2,
// 605.3a). gorge's planner never lists such a source, so a kicked total only
// its mana pays is a census gap: the play is census_incomplete and only a
// prefix search could fund it, but the source's tap queues the watcher's
// trigger above the play and a sorcery-speed cast needs an empty stack
// (CR 307.1, 602.5d); the multikicked play finds no script (witness_not_offered
// at every node) and the autopay surface drops it. An instant or flash cast
// waits for no stack, so its potential arm keeps the deferred units. hid
// reports that the sources were hidden, paid the arm's answer without them.
func (s *State) kickedPotentialSansDeferred(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost, d *payDesc) (paid, hid bool) {
	if k.gen == nil || !k.gen.anyTapsDefer || !s.subs[kop.TrigTaps].any() || k.cards[c].instant || s.castFlash(k, p, c) {
		return false, false
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		if sl := b.first(); s.perms[sl].flags&pfTapped == 0 && s.hasManaAbility(k, sl) && s.tapsDeferred(k, p, sl) {
			m = m.with(sl)
		}
	}
	if !m.any() {
		return false, false
	}
	t := s.fork()
	defer s.unfork()
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		t.perms[sl].flags |= pfTapped
		t.touchSl(sl)
	}
	return t.kickedPotentialPays(k, p, c, o, kc, d), true
}
