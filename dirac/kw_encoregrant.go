package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Araumi of the Dead Tide's pump (ExtGrantEncore; kop/ext.go): the target
// creature card in its owner's graveyard gains encore until end of turn.
// gorge's pump registers the derived keyword [Encore:CardManaCost] on the
// card while the pump lasts (CR 702.164a's grant rider is an until-end-of-turn
// effect; gorge pump.go's zone guard skips a card that left the pumped zone
// before resolution). gorge offers no activation of a granted encore --
// kw_granted.go synthesizes granted-keyword abilities for Cycling, Crew,
// Equip and friends but not Encore, and no graveyard walk offers one -- so
// the kernel records the grant alone: one mech bit on the card, part of its
// view, ending at the zone change (a new object) or the cleanup, like a
// granted flashback. A second grant of the same card holds one bit (gorge
// would list the keyword once per resolution; nothing reads the grant and a
// graveyard card's derived keywords are not projected, so the count cannot
// be observed).

// grantEncore is ExtGrantEncore.
//
//go:noinline
func (s *State) grantEncore(k *prep, in *kop.Instr) {
	if o := s.objRefObj(in.Obj); o != NoObj && s.objs[o].zone == kop.ZoneGraveyard {
		if i := k.encoreIx(); int(i) < len(s.cold.mo) {
			s.cold.mo[i].set(o)
			s.touch(o) // the grant is part of the card's view
		}
	}
}

// encoreIx is mechEncore's arena index; arenaNone when the table grants no
// encore.
func (k *prep) encoreIx() uint8 {
	if k.gen == nil {
		return arenaNone
	}
	return k.gen.ix[mechEncore]
}

// encoreGranted reports whether o holds a granted encore.
func (s *State) encoreGranted(k *prep, o Obj) bool {
	i := k.encoreIx()
	return int(i) < len(s.cold.mo) && s.cold.mo[i].has(o)
}

// encoreClear ends a granted encore on o (a zone change: a new object, CR
// 400.7). The caller marks o changed when the clear ends an actual grant.
func (s *State) encoreClear(k *prep, o Obj) {
	if i := k.encoreIx(); int(i) < len(s.cold.mo) {
		s.cold.mo[i].clr(o)
	}
}

// encoreClearAll ends every granted encore at the cleanup, marking each card
// whose grant this clear ends before dropping the bitset (fbClearAll).
func (s *State) encoreClearAll(k *prep) {
	if i := k.encoreIx(); int(i) < len(s.cold.mo) {
		s.touchBits(&s.cold.mo[i])
		s.cold.mo[i] = objBits{}
	}
}
