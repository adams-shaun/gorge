package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Remembered$CardManaCost over a permanent a RememberSacrificed$ Sacrifice took
// face down (a manifested card, a morph): the sacrificed permanent was a
// mana-value-0 object as it left the battlefield (CR 708.2), and the read goes
// by that last-known information (CR 608.2h; gorge's c.Snap.ChangeZone
// snapshot), not the printed card it is in the graveyard. mechRemFaceDown
// carries the face-down fact across the zone change, set (or cleared) by
// sacRemember as the permanent leaves.

func init() {
	arenaDetect(mechRemFaceDown, func(_ *kop.Table, in *kop.Instr) bool {
		return in.Op == kop.OpSacrifice && in.B.N&1 != 0 // seat-ok: an instruction flag bit
	})
}

// remMarkFaceDown records whether the permanent in slot sl, being sacrificed
// into the remembered set, is face down as it leaves.
func (s *State) remMarkFaceDown(k *prep, sl Slot) {
	if o := s.perms[sl].obj; s.faceDown(k, sl) {
		s.mObjSet(k, mechRemFaceDown, o)
	} else {
		s.mObjClr(k, mechRemFaceDown, o)
	}
}

// remFaceDownLeft reports whether remembered object o, off the battlefield,
// left it face down as a sacrificed permanent (remMarkFaceDown).
func (s *State) remFaceDownLeft(k *prep, o Obj) bool {
	return s.objs[o].zone != kop.ZoneBattlefield && s.mObj(k, mechRemFaceDown, o)
}
