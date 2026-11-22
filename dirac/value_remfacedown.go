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
	arenaDetect(mechRemFaceDown, func(t *kop.Table, in *kop.Instr) bool {
		if in.Op != kop.OpSacrifice || in.B.N&1 == 0 { // seat-ok: an instruction flag bit
			return false
		}
		c := remSacCard(t, in)
		return c >= 0 && remCardReadsMV(t, c)
	})
}

// remSacCard is the index of the card whose program holds in, -1 when none.
// Cold: it runs once per RememberSacrificed$ Sacrifice of a table.
func remSacCard(t *kop.Table, in *kop.Instr) int {
	for c := range t.Cards {
		for j := range t.Cards[c].Abilities {
			p := t.Cards[c].Abilities[j].Prog
			for k := range p {
				if &p[k] == in {
					return c
				}
			}
		}
	}
	return -1
}

// remCardReadsMV reports whether some ability of card c reads the remembered
// set's mana value (Remembered$CardManaCost), the only reader of the
// face-down mark: a pool whose remembering sacrificers never read it keeps no
// mask (the State bar, TestEveryPoolFitsTheStateBar: edh sits on it).
func remCardReadsMV(t *kop.Table, c int) bool {
	for j := range t.Cards[c].Abilities {
		for _, in := range t.Cards[c].Abilities[j].Prog {
			if remMVValue(t, in.A, 0) || remMVValue(t, in.B, 0) || remMVFilter(t, in.F, 0) {
				return true
			}
		}
	}
	return false
}

// remMVValue reports whether v is, or is built from, a read of the
// remembered set's mana value.
func remMVValue(t *kop.Table, v kop.ValueRef, depth int) bool {
	if v.K != kop.VExpr || depth > 4 {
		return false
	}
	x := &t.Exprs[v.N]
	return x.Kind == kop.ExMVOf && x.Ref == kop.RefRemembered ||
		remMVValue(t, x.Sub[0], depth+1) || remMVValue(t, x.Sub[1], depth+1) || remMVFilter(t, x.F, depth+1)
}

// remMVFilter is remMVValue over a filter's value bounds.
func remMVFilter(t *kop.Table, f kop.FilterID, depth int) bool {
	if f == 0 || depth > 4 {
		return false
	}
	fl := &t.Filters[f]
	return remMVValue(t, fl.PredV, depth) || remMVValue(t, fl.PickSumMax, depth)
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
