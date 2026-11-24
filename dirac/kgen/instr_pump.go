package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for pumps and keyword, colour and text grants (instr.go).
func init() {
	instrOp("PumpUntilEOT", (*instrCtx).opPumpUntilEOT)
	instrOp("GrantKeyword", (*instrCtx).opGrantKeyword)
	instrOp("PumpAll", (*instrCtx).opPumpAll)
	instrOp("GrantKwAll", (*instrCtx).opGrantKwAll)
	instrOp("PumpAllEach", (*instrCtx).opPumpAllEach)
	instrOp("ExHaste", (*instrCtx).opExHaste)
	instrOp("AsgnGrant", (*instrCtx).opAsgnGrant)
	instrOp("ColorGrant", (*instrCtx).opColorGrant)
	instrOp("KwMark", (*instrCtx).opKwMark)
	instrOp("ChangeText", (*instrCtx).opChangeText)
	instrOp("PlotGrant", (*instrCtx).opPlotGrant)
	instrOp("GrantExile", (*instrCtx).opGrantExile)
	instrOp("GrantPanharm", (*instrCtx).opGrantPanharm)
	instrOp("GrantEncore", (*instrCtx).opGrantEncore)
	instrOp("GrantEmbalmOwnCost", (*instrCtx).opGrantEmbalmOwnCost)
}

func (ic *instrCtx) opPumpUntilEOT() error {
	ki := &ic.ki
	ki.Op = kop.OpPumpUntilEOT
	return nil
}

func (ic *instrCtx) opGrantKeyword() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpGrantKeyword
	m, err := kwMask(strings.Split(in.CS, "&"))
	if err != nil {
		return err
	}
	ki.C = int16(uint16(m))
	ki.B = kop.ValueRef{K: kop.VConst, N: int16(uint16(m >> 16))}
	return nil
}

func (ic *instrCtx) opPumpAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpPumpAll
	if in.CS != "" {
		m, err := kwMask(strings.Split(in.CS, "&"))
		if err != nil {
			return err
		}
		if m>>16 != 0 {
			return fmt.Errorf("PumpAll keyword above bit 15")
		}
		ki.C = int16(uint16(m))
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opGrantKwAll() error {
	ki := &ic.ki
	in := &ic.in
	var err error
	// OpExt ExtGrantKwAll: a keyword-only PumpAll above bit 15 (lower_pumpallhighkw.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtGrantKwAll))
	if ki.C, ki.B, err = kwMaskOperands(in.CS); err != nil {
		return err
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opPumpAllEach() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.Obj = kop.OpPumpAllEach, kop.Ref(in.C) // kop.PumpEach* bits
	if in.CS != "" {
		m, err := kwMask(strings.Split(in.CS, "&"))
		if err != nil {
			return err
		}
		if m>>16 != 0 {
			return fmt.Errorf("PumpAllEach keyword above bit 15")
		}
		ki.C = int16(uint16(m))
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opExHaste() error {
	ki := &ic.ki
	in := &ic.in
	// Agility Bobblehead: OpExt ExtExHaste, C the power form (Instr.C is int16)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExHaste)), int16(in.C)
	if in.W.Kind != rNone {
		// the named-blocker form (Duct Crawler): Obj the blocker, W the
		// attacker (dirac exCNoBlock)
		ki.C |= 1 << 9
		r, err := ic.cb.ref(in.W)
		if err != nil {
			return err
		}
		ki.W = r
		return nil
	}
	if in.F != nil {
		// the filter form (Tower of Coireall, Doorman): C bit 8, F the
		// forbidden blockers' battlefield filter (dirac exCFilter)
		ki.C |= 1 << 8
		return ic.objFilter(kop.ZoneBattlefield)
	}
	return nil
}

func (ic *instrCtx) opAsgnGrant() error {
	ki := &ic.ki
	in := &ic.in
	// (Plagon, Outmaneuver) OpExt ExtAsgnGrant, C 0 toughness / 1 as-unblocked
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAsgnGrant)), int16(in.C)
	return nil
}

func (ic *instrCtx) opColorGrant() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Skrelv) OpExt ExtColorGrant, B the toxic amount
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtColorGrant)), v, int16(in.C)
	if in.F != nil {
		// (Akroma's Blessing) the sweep's filter
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opKwMark() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtKwMark, C the keywords (kop.KwMarkIntimidate |
	// kop.KwMarkInfect), Obj the subject (RefNone with F: every
	// battlefield permanent matching F; lower_kwmark.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtKwMark)), int16(in.C)
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opChangeText() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtChangeText, Obj the one target, C the word list (textreach.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChangeText)), int16(in.C)
	return nil
}

func (ic *instrCtx) opPlotGrant() error {
	ki := &ic.ki
	// (Kellan Joins Up) OpExt ExtPlotGrant, Obj the
	// Remembered ref
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPlotGrant))
	return nil
}

func (ic *instrCtx) opGrantExile() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Q, Stacking the Deck) OpExt ExtGrantExile: every remembered exiled
	// card gains the latent activated ability C (table index)
	j, ok := cb.abMap[in.C]
	if !ok {
		return fmt.Errorf("granted exile ability %d not emitted", in.C)
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtGrantExile)), int16(j)
	return nil
}

func (ic *instrCtx) opGrantPanharm() error {
	ki := &ic.ki
	// (Lwaxana Troi) OpExt ExtGrantPanharm on the target
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtGrantPanharm))
	return nil
}

func (ic *instrCtx) opGrantEncore() error {
	ki := &ic.ki
	// (Araumi of the Dead Tide) OpExt ExtGrantEncore on the target: the
	// graveyard card gains encore until end of turn; gorge offers no
	// activation of a granted encore, so the grant is the whole effect
	// (kw_encoregrant.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtGrantEncore))
	return nil
}

func (ic *instrCtx) opGrantEmbalmOwnCost() error {
	ki := &ic.ki
	// (Cursecloth Wrappings) OpGrantEmbalmOwnCost on the target; C is the
	// index of the card's latent embalm body (dirac kw_embalmgrant.go)
	ki.Op, ki.C = kop.OpGrantEmbalmOwnCost, int16(ic.in.C)
	return nil
}
