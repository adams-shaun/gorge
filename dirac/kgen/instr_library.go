package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for draw, discard, mill and library looks (instr.go).
func init() {
	instrOp("Draw", (*instrCtx).opDraw)
	instrOp("Discard", (*instrCtx).opDiscard)
	instrOp("DiscardUnless", (*instrCtx).opDiscardUnless)
	instrOp("Mill", (*instrCtx).opMill)
	instrOp("Surveil", (*instrCtx).opSurveil)
	instrOp("Scry", (*instrCtx).opScry)
	instrOp("Seek", (*instrCtx).opSeek)
	instrOp("Hideaway", (*instrCtx).opHideaway)
	instrOp("DrawUpto", (*instrCtx).opDrawUpto)
	instrOp("Explore", (*instrCtx).opExplore)
	instrOp("Discover", (*instrCtx).opDiscover)
}

func (ic *instrCtx) opDraw() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpDraw
	if err := remember3(in.CS, ki); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opDiscard() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpDiscard
	switch in.CS {
	case "DiscarderChooses":
		ki.C = kop.ChooseSelf
	case "YouChoose":
		ki.C = kop.ChooseOpp
	case "Random":
		ki.C = kop.ChooseRandom
	default:
		return fmt.Errorf("discard chooser %q", in.CS)
	}
	ki.C |= int16(in.C) // in.C: kop.OptAsk / OptAny / OptAll
	if in.Note == noteRememberDiscarded {
		ki.B = kop.Const(1) // kop.DiscRemember
	}
	if err := ic.objFilter(kop.ZoneHand); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opDiscardUnless() error {
	ki := &ic.ki
	ki.Op = kop.OpDiscardUnless
	if err := ic.objFilter(kop.ZoneHand); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opMill() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	for k := range fl {
		switch k {
		case "Remember":
			ki.C |= kop.MvRemember
		case "Optional":
			ki.C |= kop.MvMayMill
		case "ImprintMove":
			ki.C |= kop.MvImprint
		default:
			return fmt.Errorf("mill %s", in.CS)
		}
	}
	ki.Op = kop.OpMill
	return nil
}

func (ic *instrCtx) opSurveil() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpSurveil
	if in.CS == "RememberKept" {
		ki.C |= kop.SurveilRememberKept
	} else if err := remember3(in.CS, ki); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opScry() error {
	ki := &ic.ki
	ki.Op = kop.OpScry
	return nil
}

func (ic *instrCtx) opSeek() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpSeek, int16(in.C)
	if err := ic.objFilter(kop.ZoneLibrary); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opHideaway() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Collector's Cage) OpExt ExtHideaway, B the looked-at count (C 0)
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtHideaway)), v, int16(in.C)
	return nil
}

func (ic *instrCtx) opDrawUpto() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtDrawUpto: Obj the drawer, B the most cards (act_drawupto.go)
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtDrawUpto)), v
	return nil
}

func (ic *instrCtx) opExplore() error {
	ki := &ic.ki
	ki.Op = kop.OpExplore
	return nil
}

func (ic *instrCtx) opDiscover() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpDiscover, int16(in.C)
	return nil
}
