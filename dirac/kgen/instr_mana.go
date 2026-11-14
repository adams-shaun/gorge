package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for mana (instr.go).
func init() {
	instrOp("ManaChoice", (*instrCtx).opManaChoice)
	instrOp("AddMana", (*instrCtx).opAddMana)
}

func (ic *instrCtx) opManaChoice() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Chandra, Hope's Beacon) OpExt ExtManaChoice, B the amount,
	// C the colour mask (bit 8: Produced$ Any, one pick for the whole amount; bit 9: PersistentMana$ True)
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtManaChoice)), v, int16(int(colorMask(in.CS))|in.C<<8)
	if isConstN(in.B, 2) {
		ki.C |= 0x200 // PersistentMana$ True: each pick is tallied in State.pm (mana_persistent.go)
	}
	if in.F != nil {
		// RestrictValid$ Spell.<spec>: each pick floats as restricted mana
		if ki.F, err = cb.filter(in.F, kop.ZoneStack); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opAddMana() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	var err error
	ki.Op, ki.C = kop.OpAddMana, int16(colorMask(in.CS))
	if isConstN(in.B, 1) {
		ki.B = kop.Const(1) // firebending: the red units are tallied in State.fbR (kw_firebend.go)
	} else if isConstN(in.B, 2) {
		ki.B = kop.Const(2) // PersistentMana$ True: tallied in State.pm (mana_persistent.go)
	} else if isConstN(in.B, 3) {
		ki.B = kop.Const(3) // Effect UnspentMana Red: no units, keeps the controller's red (dirac/value_fightexcess.go)
	}
	if in.F != nil {
		// a loyalty ability's spend restriction: the units float as
		// restricted mana (mana_resolverestriction.go)
		if ki.F, err = cb.filter(in.F, kop.ZoneStack); err != nil {
			return err
		}
	}
	return nil
}
