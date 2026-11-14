package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for life, poison, energy and mana drain (instr.go).
func init() {
	instrOp("GainLife", (*instrCtx).opGainLife)
	instrOp("LoseLife", (*instrCtx).opLoseLife)
	instrOp("GainEnergy", (*instrCtx).opGainEnergy)
	instrOp("DrainMana", (*instrCtx).opDrainMana)
	instrOp("Poison", (*instrCtx).opPoison)
	instrOp("Blight", (*instrCtx).opBlight)
	instrOp("Extort", (*instrCtx).opExtort)
	instrOp("SetLife", (*instrCtx).opSetLife)
}

func (ic *instrCtx) opGainLife() error {
	ki := &ic.ki
	ki.Op = kop.OpGainLife
	return nil
}

func (ic *instrCtx) opLoseLife() error {
	ki := &ic.ki
	ki.Op = kop.OpLoseLife
	return nil
}

func (ic *instrCtx) opGainEnergy() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpGainEnergy, int16(in.C) // C: kop.PlCtrEnergy / kop.PlCtrExperience
	return nil
}

func (ic *instrCtx) opDrainMana() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDrainMana))
	return nil
}

func (ic *instrCtx) opPoison() error {
	ki := &ic.ki
	ki.Op = kop.OpPoison
	return nil
}

func (ic *instrCtx) opBlight() error {
	ki := &ic.ki
	ki.Op = kop.OpBlight
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opExtort() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExtort)) // dirac/act_extort.go
	return nil
}

func (ic *instrCtx) opSetLife() error {
	ki := &ic.ki
	ki.Op = kop.OpSetLife
	return nil
}
