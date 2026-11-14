package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for tapping, untapping and combat requirements (instr.go).
func init() {
	instrOp("UntapChoose", (*instrCtx).opUntapChoose)
	instrOp("UntapAll", (*instrCtx).opUntapAll)
	instrOp("TapAll", (*instrCtx).opTapAll)
	instrOp("Saddle", (*instrCtx).opSaddle)
	instrOp("Goad", (*instrCtx).opGoad)
	instrOp("Tap", (*instrCtx).opTap)
	instrOp("Untap", (*instrCtx).opUntap)
	instrOp("EnterTapped", (*instrCtx).opEnterTapped)
	instrOp("EnterUntapped", (*instrCtx).opEnterUntapped)
	instrOp("EnterAttacking", (*instrCtx).opEnterAttacking)
	instrOp("AttacksIfAble", (*instrCtx).opAttacksIfAble)
	instrOp("BlocksIfAble", (*instrCtx).opBlocksIfAble)
	instrOp("RemoveFromCombat", (*instrCtx).opRemoveFromCombat)
	instrOp("MustBeBlocked", (*instrCtx).opMustBeBlocked)
	instrOp("Suspect", (*instrCtx).opSuspect)
	instrOp("SkipNextUntap", (*instrCtx).opSkipNextUntap)
	instrOp("AtkPWGuard", (*instrCtx).opAtkPWGuard)
	instrOp("TapOrUntap", (*instrCtx).opTapOrUntap)
	instrOp("TapOrUntapAll", (*instrCtx).opTapOrUntapAll)
	instrOp("SkipUntap", (*instrCtx).opSkipUntap)
}

func (ic *instrCtx) opUntapChoose() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpUntapChoose, int16(in.C)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opUntapAll() error {
	ki := &ic.ki
	ki.Op = kop.OpUntapAll
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opTapAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpTapAll, int16(in.C) // C 1: TapperController$ True (act_sweeps.go tapAll)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opSaddle() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpSaddle, int16(in.C) // C = 1: Solved
	return nil
}

func (ic *instrCtx) opGoad() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpGoad
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opTap() error {
	ki := &ic.ki
	ki.Op = kop.OpTap
	return nil
}

func (ic *instrCtx) opUntap() error {
	ki := &ic.ki
	ki.Op = kop.OpUntap
	return nil
}

func (ic *instrCtx) opEnterTapped() error {
	ki := &ic.ki
	ki.Op = kop.OpEnterTapped
	return nil
}

func (ic *instrCtx) opEnterUntapped() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEnterUntapped))
	return nil
}

func (ic *instrCtx) opEnterAttacking() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEnterAttacking))
	return nil
}

func (ic *instrCtx) opAttacksIfAble() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtAttacksIfAble, Obj the target that attacks this turn if able
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAttacksIfAble)), int16(in.C)
	return nil
}

func (ic *instrCtx) opBlocksIfAble() error {
	ki := &ic.ki
	// OpExt ExtBlocksIfAble, Obj the target that blocks this turn if able
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBlocksIfAble))
	return nil
}

func (ic *instrCtx) opRemoveFromCombat() error {
	ki := &ic.ki
	// OpExt ExtRemoveFromCombat, Obj the one object removed (lower_removefromcombat.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRemoveFromCombat))
	return nil
}

func (ic *instrCtx) opMustBeBlocked() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtMustBeBlocked, Obj the subject (RefNone
	// with F: every battlefield permanent matching F)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMustBeBlocked))
	if in.C == kop.MustBeBlockedLure {
		ki.C = kop.MustBeBlockedLure // the lure mark (a single Obj)
	}
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opSuspect() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtSuspect, Obj the subject (RefNone with F: every
	// battlefield permanent matching F), C bit 0 the Activate$ False removal
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSuspect)), int16(in.C&1)
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opSkipNextUntap() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpSkipNextUntap
	if in.F != nil {
		// the sweep form (Sleep): every battlefield
		// permanent matching F, the player Obj's when Obj is set
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opAtkPWGuard() error {
	ki := &ic.ki
	ki.Op = kop.OpAtkPWGuard
	return nil
}

func (ic *instrCtx) opTapOrUntap() error {
	ki := &ic.ki
	ki.Op = kop.OpTapOrUntap
	return nil
}

func (ic *instrCtx) opTapOrUntapAll() error {
	ki := &ic.ki
	// OpExt ExtTapOrUntapAll over the sweep filter (act_taporuntapall.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtTapOrUntapAll))
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opSkipUntap() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpSkipUntap, int16(in.C)
	return nil
}
