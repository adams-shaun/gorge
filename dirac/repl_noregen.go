package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The can't-be-regenerated mark (CR 701.19c): a creature dealt damage by a
// DmgNoRegen damage instruction (Incinerate: "a creature dealt damage this
// way can't be regenerated this turn") keeps its regeneration shields but
// never spends one, so a destruction or lethal damage destroys it. The mark
// is a slot bit in the arena's mechNoRegen mask (0 B of State): it ends at the
// turn's cleanup, and when the slot takes a new permanent (arenaClrSlot),
// since the effect is forgotten as the creature leaves the battlefield.

// markNoRegen marks the permanent o on the battlefield when it is a creature
// (gorge's follow-up Effect remembers Remembered.Creature, the creatures
// the damage reached above 0).
func (s *State) markNoRegen(k *prep, o Obj) {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && s.isCreature(k, ob.slot) {
		s.mSlSet(k, mechNoRegen, ob.slot)
	}
}

// canRegen reports whether the permanent at sl has a regeneration shield it
// may spend: one is there and no can't-be-regenerated mark is on it.
func (s *State) canRegen(sl Slot) bool {
	return s.perms[sl].flags&pfRegen != 0 && !s.mSl(s.k(), mechNoRegen, sl)
}
