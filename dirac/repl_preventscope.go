package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNoPrevScope (the scoped-prevention lockstep mutation): the scoped
// records are made and ordered but prevent nothing.
var DebugNoPrevScope bool

// prevScopeAdd is Ext PreventDamageScope (CR 615.1): the resolving Effect
// registers a this-turn prevention of the damage that would be dealt to its
// controller (kop.PrevScopePlayer) and/or the creatures that player controls
// (kop.PrevScopeCreatures), narrowed to creature sources on the battlefield
// by kop.PrevScopeSrcCreature (gorge effEffect's bodyless DamageDone Prevent$
// replacement in the Command zone). It is one more record of genState.prevCol,
// so each is its own CR 616.1 order candidate beside the others; a fourth in
// one turn stalls loudly (StallPrevCol).
func (s *State) prevScopeAdd(k *prep, in *kop.Instr) {
	sc := uint8(in.C)
	for i := range s.g.prevCol {
		r := &s.g.prevCol[i]
		if r.col == 0 && r.sc == 0 {
			r.obj, r.sc, r.ctl = s.res.e.obj, sc, s.res.e.ctrl
			return
		}
	}
	s.capFull(StallPrevCol) // loud: never drop an effect
}

// prevScopeHit reports whether a record of scope sc made by player ctl
// prevents damage from the source at srcSl to t.
func (s *State) prevScopeHit(k *prep, sc, ctl uint8, srcSl Slot, t prevTgt) bool {
	if DebugNoPrevScope || t.rcp != ctl {
		return false
	}
	if sc&kop.PrevScopeSrcCreature != 0 && (srcSl == NoSlot || !s.isCreature(k, srcSl)) {
		return false
	}
	if t.dst == NoSlot {
		return sc&kop.PrevScopePlayer != 0
	}
	return sc&kop.PrevScopeCreatures != 0 && s.isCreature(k, t.dst)
}

// recipientCtl is the controller of a combat damage recipient: the player
// code (an attacker's damage to the player it attacks) when dst is NoSlot.
func (s *State) recipientCtl(dst Slot, code uint8) uint8 {
	if dst == NoSlot {
		return CodeSeat(code)
	}
	return s.perms[dst].ctrl
}
