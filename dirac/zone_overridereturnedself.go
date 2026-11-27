package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// overrideReturnedSelf is the slot an override reads for "Defined$ Self" right
// after the same ability returned its own source from the graveyard
// (Llanowar Greenwidow's "return this card from your graveyard to the
// battlefield tapped. It gains ..."). The return is a zone change, so the
// permanent is a new object (objRef holds Self to the activation's version);
// gorge's sub-ability follows the card it just moved, so the override applies
// to the returned permanent. A second activation of the same graveyard
// ability whose return finds the card already back (a new object, so the
// return moves nothing: execReturnObj) still runs the animate on that
// permanent in gorge, so it is a second record there too (two Replacements$
// lines compete: dieZone's StallReplLeave). It is NoSlot unless the previous
// instruction is that Self return.
func (s *State) overrideReturnedSelf(k *prep, in *kop.Instr) Slot {
	if in.Obj != kop.RefSelf || s.res.pc == 0 {
		return NoSlot
	}
	e := &s.res.e
	if e.kind != seAbility || s.activatedFrom(k, e) != kop.ZoneGraveyard {
		return NoSlot
	}
	if p := &s.prog(k)[s.res.pc-1]; (p.Op != kop.OpReturn && p.Op != kop.OpReturnEx) || p.Obj != kop.RefSelf {
		return NoSlot
	}
	if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield {
		return ob.slot
	}
	return NoSlot
}
