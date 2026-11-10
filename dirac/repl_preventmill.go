package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplPreventMillOpps (The Mindskinner): damage a source its controller
// controls would deal to an opponent is prevented, and that opponent mills
// that many cards instead. gorge (rules/replacement.go
// applyNonMoveReplacements) runs the DB$ Mill body when the damage event is
// proposed and hands back a stopped event: nothing is dealt, so no lifelink,
// no damage trigger, no life loss. Each damage event is one application; a
// combat damage step has one event per attacker, so three unblocked
// attackers mill three times. Beside a doubler, a plus-N or a colour
// prevention gorge asks the CR 616.1 order; that pair is refused before the
// game (stallclass.go initStall), so here it is the only replacement on a
// player's damage after protection.

// preventMill applies the replacement to n damage from a source srcCtrl
// controls to player rcp, an opponent of srcCtrl: true when one of srcCtrl's
// permanents carries it (the damage is then 0). The mill happens now, as
// gorge's body runs at the proposal. A combat pass the order ask paused is
// recomputed on its resume (replOrd), so a mill there would run twice: that
// state stalls, as the order ask with this replacement is not modelled.
func (s *State) preventMill(k *prep, srcCtrl, rcp uint8, n int) bool {
	if s.replN(k, srcCtrl, rbPrevMill) == 0 {
		return false
	}
	if s.noPrev(k) {
		s.capFull(StallNoPrevent) // gorge prevents and mills anyway (static_cantprevent.go)
		return true
	}
	if s.g.ro.on || s.parkedAny() {
		s.capFull(StallReplPlus)
		return true
	}
	if k.gen.anyRepl&rbGraveLib != 0 {
		s.millShuffling(k, rcp, n, 0)
		return true
	}
	pl := &s.pl[rcp]
	for i := 0; i < n && pl.libLo < pl.libHi; i++ {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	}
	return true
}

// parkedAny reports a live parked-move record. cold.parked is allocated at a
// fixed length once the table carries a graveyard-move competition, so its
// length says nothing; unpark compacts, so a live record sits at index 0.
func (s *State) parkedAny() bool {
	return len(s.cold.parked) != 0 && s.cold.parked[0].kind != parkNone
}
