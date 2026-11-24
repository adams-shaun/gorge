package kgen

import (
	"fmt"
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for copies and clones (instr.go).
func init() {
	instrOp("CopyAllEach", (*instrCtx).opCopyAllEach)
	instrOp("CopyAbility", (*instrCtx).opCopyAbility)
	instrOp("CloneCopy", (*instrCtx).opCloneCopy)
	instrOp("AegisClone", (*instrCtx).opAegisClone)
	instrOp("CopyAsToken", (*instrCtx).opCopyAsToken)
	instrOp("CopyNamed", (*instrCtx).opCopyAsToken)
	instrOp("CopyPicked", (*instrCtx).opCopyAsToken)
	instrOp("CopyExiledWith", (*instrCtx).opCopyAsToken)
	instrOp("Encore", (*instrCtx).opCopyAsToken) // kop.ExtEncore (dirac/kw_encore.go)
	instrOp("Myriad", (*instrCtx).opCopyAsToken) // kop.ExtMyriad (dirac/kw_myriad.go)
	instrOp("CopyAsTokenCtr", (*instrCtx).opCopyAsTokenCtr)
	instrOp("BecomeCopy", (*instrCtx).opBecomeCopy)
}

func (ic *instrCtx) opCopyAllEach() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtCopyAllEach, F the Defined$ Valid set,
	// C the copy exceptions' override index+1 (as CopyAsToken)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyAllEach))
	ki.B = kop.Const(0)
	if in.C > 0 {
		i, err := cb.override(cb.c.overrides[in.C-1])
		if err != nil {
			return err
		}
		ki.C = int16(i + 1)
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opCopyAbility() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtCopyAbility, B the copy count (Amount$, default 1), C bit 0
	// the MayChooseTarget$ election and C>>1 the target slot (not Obj: a
	// target Ref would run the program once per target)
	slot, ok := tgtSlotOfRef(ki.Obj)
	if ki.Obj == kop.RefTriggerObj {
		slot, ok = kop.CopyAbTriggering, true // the ability that fired an AbilityCast trigger
	}
	if !ok {
		return fmt.Errorf("CopyAbility Obj %d", ki.Obj)
	}
	n := ki.A
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtCopyAbility)), n, int16(in.C&1)|int16(slot)<<1
	ki.Obj = kop.RefSelf
	return nil
}

func (ic *instrCtx) opCloneCopy() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtCloneCopy; B.N the kop.Clone* flags, C the
	// copy exceptions' override index+1, F the Choices$ candidates (the
	// pick) or the Valid set of becomers
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCloneCopy))
	if in.C > 0 {
		i, err := cb.override(cb.c.overrides[in.C-1])
		if err != nil {
			return err
		}
		ki.C = int16(i + 1)
		if ic.ki.B.N&kop.CloneGrant != 0 {
			// the grant-only record must be the pool record after the exceptions'
			g, err := cb.override(cb.c.overrides[in.C])
			if err != nil {
				return err
			}
			if g != i+1 {
				return fmt.Errorf("CloneCopy grant record %d does not follow the exceptions' %d", g, i)
			}
		}
	}
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opAegisClone() error {
	ki := &ic.ki
	// (Assimilation Aegis) OpExt ExtAegisClone
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtAegisClone))
	return nil
}

func (ic *instrCtx) opCopyAsToken() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	ki.Op = kop.OpCopyAsTokenEx
	cf := 0
	if _, ok := fl["SacAtEndStep"]; ok {
		cf |= kop.CpSacAtEnd
	}
	if _, ok := fl["ExileAtEndStep"]; ok {
		cf |= kop.CpExileAtEnd
	}
	if _, ok := fl["ExileAtCombat"]; ok {
		cf |= kop.CpExileAtCombat
	}
	if _, ok := fl["SacAtEndDly"]; ok {
		cf |= kop.CpSacAtEndDly
	}
	if _, ok := fl["SacAtUpkeep"]; ok {
		cf |= kop.CpSacAtUpkeep
	}
	if _, ok := fl["Remember"]; ok {
		cf |= kop.CpRemember
	}
	if _, ok := fl["Tapped"]; ok {
		cf |= kop.CpTapped
	}
	if _, ok := fl["NonRemCtrl"]; ok {
		cf |= kop.CopyNonRemCtrl
	}
	if _, ok := fl["Attacking"]; ok {
		cf |= kop.CopyAttacking // ExtCopyPicked only (lower_copypicked.go)
	}
	pk, ok := fl["Pump"]
	pe, eot := fl["PumpEOT"]
	if eot {
		pk, ok = pe, true
		cf |= kop.CopyPumpEOT
	}
	if ok {
		// PumpKeywords$: a non-copy keyword grant; the kernel finds a
		// permanent one's keyword-only override record by its mask
		m, err := kwMask([]string{pk})
		if err != nil {
			return err
		}
		if !eot {
			if _, err := cb.override(&lOverride{AddKw: []string{pk}, Dedup: true}); err != nil {
				return err
			}
		}
		cf |= (bits.TrailingZeros32(uint32(m)) + 1) << kop.CopyPumpShift
	}
	ki.B = kop.Const(cf)
	if in.C > 0 {
		// the copy exceptions' override (CR 707.9b)
		i, err := cb.override(cb.c.overrides[in.C-1])
		if err != nil {
			return err
		}
		ki.C = int16(i + 1)
	}
	switch in.Op {
	case "Encore":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEncore)) // Obj, B and C as above
	case "Myriad":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMyriad))
	}
	if in.Op == "CopyPicked" {
		// kop.ExtCopyPicked: Obj the picked card, B and C as above
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyPicked))
	}
	if in.Op == "CopyNamed" {
		// kop.ExtCopyNamed: B holds the named card's pool index (the
		// matchup remaps it to its CardID), C the flags and exception;
		// A held the count (one), now the sub-op
		idx, ok := cb.pt.tokenIdx[copyCardKey+fl["card"]]
		if !ok {
			return fmt.Errorf("copied card %q", fl["card"])
		}
		if cf := int(uint16(ki.B.N)); cf>>8 != 0 || int(ki.C) > 0xff {
			return fmt.Errorf("named copy flags %#x or exception override %d past a byte", cf, ki.C)
		}
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyNamed))
		ki.B, ki.C = kop.Const(idx), int16(int(ki.C)<<kop.CopyNamedExcShift|int(uint16(ki.B.N)))
	}
	if in.Op == "CopyExiledWith" {
		// kop.ExtCopyExiledWith: F the exiled-with set, B and C as above
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyExiledWith))
		if err := ic.objFilter(kop.ZoneExile); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opCopyAsTokenCtr() error {
	ki := &ic.ki
	in := &ic.in
	// dirac/copy_counters.go: Obj the source, B the
	// amount (set above), C the kind (not the copy-exception index: kgen
	// refuses an exception beside the rider)
	k, err := counterKind(in.CS)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCopyTokenCtr)), int16(k)
	return nil
}

func (ic *instrCtx) opBecomeCopy() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpBecomeCopy
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	if in.B.Kind != vNone {
		if !in.B.isConst() || in.B.N&^(kop.BecomeCopyCtr|kop.BecomeCopyEmbalm) != 0 || in.B.N == 0 {
			return fmt.Errorf("BecomeCopy B %v", in.B)
		}
		ki.B = kop.Const(in.B.N)
	}
	ki.C = int16(in.C & 1)
	if x := in.C >> 1; x > 0 {
		i, err := cb.override(cb.c.overrides[x-1])
		if err != nil {
			return err
		}
		if i+1 > 0x3fff {
			return fmt.Errorf("copy exception override %d", i)
		}
		ki.C |= int16(i+1) << 1
	}
	ki.A = kop.ValueRef{}
	if in.A.Kind != vNone {
		// the AddAbilities$ grant (lower_cloneentergrant.go)
		if !in.A.isConst() || in.A.N < 1 || in.A.N > len(cb.c.overrides) {
			return fmt.Errorf("BecomeCopy A %v", in.A)
		}
		i, err := cb.override(cb.c.overrides[in.A.N-1])
		if err != nil {
			return err
		}
		ki.A = kop.Const(i + 1)
	}
	return nil
}
