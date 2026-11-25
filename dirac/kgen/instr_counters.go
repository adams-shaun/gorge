package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for counters (instr.go).
func init() {
	instrOp("PutCounterAll", (*instrCtx).opPutCounterAll)
	instrOp("DoubleCountersAll", (*instrCtx).opDoubleCountersAll)
	instrOp("PutCounter", (*instrCtx).opPutCounter)
	instrOp("RemoveCounter", (*instrCtx).opPutCounter)
	instrOp("EnterWithCounters", (*instrCtx).opPutCounter)
	instrOp("Bolster", (*instrCtx).opBolster)
	instrOp("Support", (*instrCtx).opSupport)
	instrOp("Monstrosity", (*instrCtx).opMonstrosity)
	instrOp("DoubleCounters", (*instrCtx).opDoubleCounters)
	instrOp("Proliferate", (*instrCtx).opProliferate)
	instrOp("DigCounters", (*instrCtx).opDigCounters)
	instrOp("SearchCounters", (*instrCtx).opSearchCounters)
	instrOp("Cumulative", (*instrCtx).opCumulative)
	instrOp("PutCounterExisting", (*instrCtx).opPutCounterExisting)
	instrOp("AddOrRemoveCounter", (*instrCtx).opAddOrRemoveCounter)
	instrOp("RemoveCounterAny", (*instrCtx).opRemoveCounterAny)
	instrOp("DistCounters", (*instrCtx).opDistCounters)
	instrOp("Endure", (*instrCtx).opEndure)
	instrOp("EffEtbCounters", (*instrCtx).opEffEtbCounters)
	instrOp("EffFinalityCtr", (*instrCtx).opEffFinalityCtr)
	instrOp("DividedCounters", (*instrCtx).opDividedCounters)
	instrOp("CounterLose", (*instrCtx).opCounterLose)
	instrOp("Renown", (*instrCtx).opRenown)
	instrOp("CounterPlusEOT", (*instrCtx).opCounterPlusEOT)
	instrOp("MoveCounter", (*instrCtx).opMoveCounter)
	instrOp("ClearCounters", (*instrCtx).opClearCounters)
	instrOp("RemoveCounterAll", (*instrCtx).opRemoveCounterAll)
	instrOp("PutCountersFrom", (*instrCtx).opPutCountersFrom)
}

func (ic *instrCtx) opPutCounterAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpPutCounterAll
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.C = int16(k)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opDoubleCountersAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpDoubleCountersAll
	if in.CS == "All" {
		ki.C = 255
	} else {
		k, err := counterKind(in.CS)
		if err != nil {
			return err
		}
		ki.C = int16(k)
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opPutCounter() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = map[string]kop.Op{"PutCounter": kop.OpPutCounter, "RemoveCounter": kop.OpRemoveCounter, "EnterWithCounters": kop.OpEnterWithCounters}[in.Op]
	if in.Op == "RemoveCounter" && in.C == 1 {
		ki.Op = kop.OpRemoveCounterRem
	}
	if in.Op == "PutCounter" && in.CS == contractCounter {
		// a player's counter (lower_contract.go): OpExt ExtPutContract on
		// Obj (the players), B the amount (dirac/ctr_contract.go). C is the
		// armed marker armContractPut sets when the same chain carries Miss
		// Highwater's LosesGame Effect; an unarmed put records no debt.
		v, err := ic.cb.value(in.A)
		if err != nil {
			return err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtPutContract)), v, int16(in.C)
		return nil
	}
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.C = int16(k)
	return nil
}

func (ic *instrCtx) opBolster() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpBolster
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.C = int16(k)
	return nil
}

func (ic *instrCtx) opSupport() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpSupport
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.C = int16(k) | int16(in.C)<<8 // C=1: other than the source
	if in.Obj.Kind == rTarget {
		// C&0x1000: the cast announced the targets (an instant or sorcery's
		// Support, CR 601.2c); the kernel applies one counter to each target
		// of Obj's slot instead of opening a resolution pick.
		ki.C |= 0x1000
	}
	if in.C&2 != 0 { // the remove form (2 remove, 4 exact, 8 remember amount) elects over F
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opMonstrosity() error {
	ki := &ic.ki
	ki.Op = kop.OpMonstrosity
	return nil
}

func (ic *instrCtx) opDoubleCounters() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpDoubleCounters
	if in.CS == "All" {
		ki.C = 255
	} else {
		k, err := counterKind(in.CS)
		if err != nil {
			return err
		}
		ki.C = int16(k)
	}
	return nil
}

func (ic *instrCtx) opProliferate() error {
	ki := &ic.ki
	ki.Op = kop.OpProliferate
	return nil
}

func (ic *instrCtx) opDigCounters() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpDigCounters, int16(in.C)
	return nil
}

func (ic *instrCtx) opSearchCounters() error {
	ki := &ic.ki
	in := &ic.in
	// (Captain Jean-Luc Picard) OpExt ExtSearchCounters: B the amount
	// (0 clears), C the kind; kgen emits it around the Search.
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSearchCounters)), int16(in.C)
	return nil
}

func (ic *instrCtx) opCumulative() error {
	ki := &ic.ki
	in := &ic.in
	// (Mystic Remora) OpExt ExtCumulative, C the instructions skipped
	// when the source is gone (lower_cumulative.go)
	if in.C < 1 || in.C > 3 {
		return fmt.Errorf("Cumulative skip %d", in.C)
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCumulative)), int16(in.C)
	return nil
}

func (ic *instrCtx) opPutCounterExisting() error {
	ki := &ic.ki
	// (Dathon, His Heart Open) OpExt ExtPutCounterExisting; Obj and B
	// are converted above
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPutCounterExisting))
	return nil
}

func (ic *instrCtx) opRemoveCounterAny() error {
	ki := &ic.ki
	in := &ic.in
	// (Glissa Sunslayer) OpExt ExtRemoveCounterAny
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtRemoveCounterAny)), int16(in.C)
	return nil
}

func (ic *instrCtx) opDistCounters() error {
	ki := &ic.ki
	// (Vastwood Hydra) OpExt ExtDistCounters: B the total, F the pool
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDistCounters))
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opEndure() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtEndure, B the Spirit token, C the N/N
	// override (its PA is N)
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("token %q", in.CS)
	}
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtEndure)), kop.Const(idx), int16(i)
	return nil
}

func (ic *instrCtx) opEffEtbCounters() error {
	ki := &ic.ki
	// OpExt ExtEffEtbCtr, B the count (SetChosenNumber$),
	// Obj the triggering spell (the kernel reads the entry's trigObj)
	ki.Op, ki.B = kop.OpExt, ki.A
	ki.A = kop.Const(int(kop.ExtEffEtbCtr))
	return nil
}

func (ic *instrCtx) opEffFinalityCtr() error {
	ki := &ic.ki
	// OpExt ExtFinalityCtr, Obj the triggering spell
	// (the kernel reads the entry's trigObj); the spell enters with one
	// FINALITY counter per applied grant
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtFinalityCtr))
	return nil
}

func (ic *instrCtx) opDividedCounters() error {
	ki := &ic.ki
	in := &ic.in
	// (Quirion Beastcaller) OpExt ExtDividedCounters, Obj the target slot
	// (C, not Obj: a target Ref would run the program once per target)
	slot, ok := tgtSlotOfRef(ki.Obj)
	if !ok {
		return fmt.Errorf("DividedCounters Obj %d", ki.Obj)
	}
	ki.Op, ki.A, ki.C, ki.Obj = kop.OpExt, kop.Const(int(kop.ExtDividedCounters)), int16(slot), kop.RefSelf
	if in.CS != "" {
		// the literal-N form's CounterType$ (Contagion's M2M1): B.N the kind
		k, err := counterKind(in.CS)
		if err != nil {
			return err
		}
		ki.B = kop.Const(int(k))
	}
	return nil
}

func (ic *instrCtx) opCounterLose() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Tishana's Tidebinder) OpExt ExtCounterLose, C the
	// host-bound LoseAbilities override
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCounterLose)), int16(i)
	return nil
}

func (ic *instrCtx) opRenown() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtRenown on Obj (the source), B the counter count (dirac/act_renown.go)
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtRenown)), v
	return nil
}

func (ic *instrCtx) opCounterPlusEOT() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCounterPlusEOT))
	return nil
}

func (ic *instrCtx) opMoveCounter() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpMoveCounter
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.B = kop.Const(int(k))
	r, err := cb.ref(in.Obj2)
	if err != nil {
		return err
	}
	ki.C = int16(r)
	return nil
}

func (ic *instrCtx) opClearCounters() error {
	ki := &ic.ki
	ki.Op = kop.OpClearCounters
	return nil
}

func (ic *instrCtx) opRemoveCounterAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpRemoveCounterAll
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.C = int16(k)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opPutCountersFrom() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpPutCountersFrom
	r, err := cb.ref(in.Obj2)
	if err != nil {
		return err
	}
	ki.C = int16(r)
	return nil
}
