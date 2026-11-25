package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for control flow: guards, modes, branches, loops and turn structure (instr.go).
func init() {
	instrOp("SkipIf", (*instrCtx).opSkipIf)
	instrOp("Charm", (*instrCtx).opCharm)
	instrOp("Branch", (*instrCtx).opBranch)
	instrOp("RepeatEachCard", (*instrCtx).opRepeatEachCard)
	instrOp("RepeatEach", (*instrCtx).opRepeatEach)
	instrOp("CopySpell", (*instrCtx).opCopySpell)
	instrOp("Demonstrate", (*instrCtx).opDemonstrate)
	instrOp("Ninjutsu", (*instrCtx).opNinjutsu)
	instrOp("Repeat", (*instrCtx).opRepeat)
	instrOp("RepeatWhile", (*instrCtx).opRepeatWhile)
	instrOp("GenericChoice", (*instrCtx).opGenericChoice)
	instrOp("NumTimes", (*instrCtx).opNumTimes)
	instrOp("AddPhase", (*instrCtx).opAddPhase)
	instrOp("SkipPhase", (*instrCtx).opSkipPhase)
	instrOp("LookAck", (*instrCtx).opLookAck)
	instrOp("MayAsk", (*instrCtx).opMayAsk)
	instrOp("AtEOT", (*instrCtx).opAtEOT)
	instrOp("LoseGame", (*instrCtx).opLoseGame)
	instrOp("ChooseTargets", (*instrCtx).opChooseTargets)
	instrOp("ExtJoinForces", (*instrCtx).opExtJoinForces)
	instrOp("AddTurn", (*instrCtx).opAddTurn)
	instrOp("EndTurn", (*instrCtx).opEndTurn)
}

func (ic *instrCtx) opSkipIf() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpSkipUnless
	id, err := cb.cond(in.Cond)
	if err != nil {
		return err
	}
	ki.C, ki.A = int16(id), ic.skip()
	return nil
}

func (ic *instrCtx) opCharm() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C, ki.A = kop.OpCharm, int16(in.C), ic.skip()
	return nil
}

func (ic *instrCtx) opBranch() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C, ki.A = kop.OpBranch, int16(in.C), ic.skip()
	return nil
}

func (ic *instrCtx) opRepeatEachCard() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.B, ki.C = kop.OpRepeatEach, ic.skip(), int16(in.C) // act_repeatcards.go; C RepeatPlain
	if in.F != nil {
		if err := ic.objFilter(zBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opRepeatEach() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A, ki.C = kop.OpRepeatEachPlayer, ic.skip(), int16(in.C)
	if in.F != nil {
		if err := ic.objFilter(zBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opCopySpell() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpCopySpell, int16(in.C)
	if in.B.isConst() && in.B.N > 0 {
		// NonLegendary$'s copy exception (lower_effect_copy.go copyNonLegendary)
		i, err := ic.cb.override(ic.cb.c.overrides[in.B.N-1])
		if err != nil {
			return err
		}
		ki.B = kop.Const(i + 1)
	}
	return nil
}

func (ic *instrCtx) opDemonstrate() error {
	ki := &ic.ki
	ki.Op = kop.OpDemonstrate
	return nil
}

func (ic *instrCtx) opNinjutsu() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpNinjutsu, int16(in.C) // C 1: the plain put-self-from-hand (lower_ninjutsu.go putSelfAbility)
	return nil
}

func (ic *instrCtx) opRepeat() error {
	ki := &ic.ki
	ki.Op, ki.B = kop.OpRepeat, ic.skip() // A: the pass count, read as the loop starts
	return nil
}

func (ic *instrCtx) opRepeatWhile() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	id, err := cb.cond(in.Cond)
	if err != nil {
		return err
	}
	if id == 0 {
		return fmt.Errorf("RepeatWhile without a condition")
	}
	ki.Op, ki.B, ki.C = kop.OpRepeatWhile, ic.skip(), int16(id)
	return nil
}

func (ic *instrCtx) opGenericChoice() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpChooseBranch, int16(in.C) // C 1: no payability filter (lower_genericchoice.go genericChoiceYou)
	return nil
}

func (ic *instrCtx) opNumTimes() error {
	ki := &ic.ki
	in := &ic.in
	// (Tranquil Frillback) OpExt ExtNumTimes, B the unit's packed
	// pips; C = the unit's generic | the announcement maximum << 4 | the
	// decline jump << 7
	if ki.A.K != kop.VConst || in.C < 1 || in.C > 7 || ki.A.N < 0 || ki.A.N > 15 || in.Skip < 0 || in.Skip > 255 {
		return fmt.Errorf("NumTimes shape")
	}
	ki.C = ki.A.N | int16(in.C)<<4 | int16(in.Skip)<<7
	if strings.Contains(in.CS, "open") {
		ki.F = 1 // no AnnounceMax$: stall loud past the maximum
	}
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtNumTimes))
	return nil
}

func (ic *instrCtx) opAddPhase() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtAddPhase, B the count, C the kind | splice point
	ki.Op, ki.B, ki.C = kop.OpExt, ki.A, int16(in.C)
	ki.A = kop.Const(int(kop.ExtAddPhase))
	return nil
}

func (ic *instrCtx) opSkipPhase() error {
	ki := &ic.ki
	// OpExt ExtSkipPhase, Obj the skipping player(s), C the shape (dirac/act_skipphase.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSkipPhase)), int16(ic.in.C)
	return nil
}

func (ic *instrCtx) opLookAck() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpLookAck
	if in.CS == "Library" {
		ki.C = 1 // the top A cards of the library (Mishra's Bauble)
	}
	return nil
}

func (ic *instrCtx) opMayAsk() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C, ki.A = kop.OpMayAsk, int16(in.C), ic.skip()
	return nil
}

func (ic *instrCtx) opAtEOT() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpAtEOT, int16(in.C)
	return nil
}

func (ic *instrCtx) opLoseGame() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpLoseGame, int16(in.C) // in.C 1: WinsGame (kernel winGame)
	return nil
}

func (ic *instrCtx) opChooseTargets() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpChooseTargets
	ki.C = int16(in.C)
	return nil
}

func (ic *instrCtx) opExtJoinForces() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtJoinForces)), int16(in.C) // lower_storesvar.go
	return nil
}

func (ic *instrCtx) opAddTurn() error {
	ki := &ic.ki
	ki.Op = kop.OpAddTurn
	if _, ok := ic.fl["ExtraTurnRider"]; ok {
		// the granted turn's ExtraTurnDelayedTrigger rider (Final Fortune):
		// interp arms it when the granted turn begins and its end step fires
		// the loss (act_extraturn.go). The end step is the only lowered phase.
		ki.C = int16(kop.StepEnd)
	}
	return nil
}

func (ic *instrCtx) opEndTurn() error {
	ki := &ic.ki
	ki.Op = kop.OpEndTurn
	return nil
}
