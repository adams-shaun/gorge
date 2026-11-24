package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for tokens and face-down creatures (instr.go).
func init() {
	instrOp("Amass", (*instrCtx).opAmass)
	instrOp("Empower", (*instrCtx).opEmpower)
	instrOp("Manifest", (*instrCtx).opManifest)
	instrOp("Cloak", (*instrCtx).opCloak)
	instrOp("ManifestDread", (*instrCtx).opManifestDread)
	instrOp("Incubate", (*instrCtx).opIncubate)
	instrOp("CreateToken", (*instrCtx).opCreateToken)
	instrOp("TokenPT", (*instrCtx).opTokenPT)
	instrOp("Recruit", (*instrCtx).opRecruit)
}

func (ic *instrCtx) opAmass() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpAmass
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("token %q", in.CS)
	}
	ki.C = int16(idx)
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	if i+1 > 0xfff {
		return fmt.Errorf("amass override index %d", i)
	}
	ki.B = kop.Const(i + 1)
	if in.B.N != 0 {
		ki.B.N |= -0x8000 // bit 15: RememberAmass$
	}
	return nil
}

func (ic *instrCtx) opEmpower() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpEmpower
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("token %q", in.CS)
	}
	ki.C = int16(idx)
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	if i+1 > 0xfff {
		return fmt.Errorf("empower override index %d", i)
	}
	ki.B = kop.Const(i + 1)
	return nil
}

func (ic *instrCtx) opManifest() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpManifest
	if in.C != 0 {
		ki.B = kop.Const(in.C) // CfManifestRemembered
	}
	return nil
}

func (ic *instrCtx) opCloak() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpCloak
	ki.B = kop.Const(in.C)
	return nil
}

func (ic *instrCtx) opManifestDread() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpManifestDread
	ki.B = kop.Const(in.C)
	return nil
}

func (ic *instrCtx) opIncubate() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpIncubate
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("token %q", in.CS)
	}
	ki.C = int16(idx)
	return nil
}

func (ic *instrCtx) opCreateToken() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpCreateTokenEx
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("token %q", in.CS)
	}
	ki.C = int16(idx)
	if in.C&(6|24|32|64|128|256|512|1024|2048|tokRemSrcBit) == 0 { // tokListBit rides the core op
		// the core's CreateToken: B.N = TokenRider flags.
		ki.Op = kop.OpCreateToken
		b := 0
		if in.C&1 != 0 {
			b |= kop.TrTapped
		}
		if in.C&tokListBit != 0 {
			b |= kop.TrList
		}
		if b != 0 {
			ki.B = kop.Const(b)
		}
		return nil
	}
	tf := 0
	if in.C&2 != 0 {
		tf |= kop.TokRemember
	}
	if in.C&4 != 0 {
		tf |= kop.TokRememberObj
	}
	if in.C&1 != 0 {
		tf |= kop.TokTapped
	}
	if in.C&2048 != 0 {
		tf |= kop.TokImprintObj // lower_imprintzone.go
	}
	if in.C&tokRemSrcBit != 0 {
		tf |= kop.TokRememberSrc // lower_tokenrem.go
	}
	if in.C&8 != 0 {
		tf |= kop.TokSacAtEnd
	}
	if in.C&16 != 0 {
		tf |= kop.TokExileAtEnd
	}
	if in.C&512 != 0 {
		tf |= int(kop.TokAtEOTDestroy) << 8
	}
	if in.C&1024 != 0 {
		tf |= int(kop.TokAtEOTSacCombat) << 8
	}
	if in.C&64 != 0 {
		tf |= kop.TokAttacking
	}
	if in.C&128 != 0 {
		tf |= kop.TokAttackAsk
	}
	if in.C&tokAtkPlayerBit != 0 {
		if in.C&(8|16|32|512|1024) != 0 {
			return fmt.Errorf("a player-only TokenAttacking$ rider beside an AtEOT$ or attach rider")
		}
		tf |= kop.TokAttackPlayer
	}
	if in.C&32 != 0 {
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return err
		}
		tf |= kop.TokAttach
		ki.B = kop.Const(tf | int(kop.TokAttachN(r)))
		return nil
	}
	ki.B = kop.Const(tf)
	return nil
}

func (ic *instrCtx) opTokenPT() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtTokenPT right before the CreateToken, C the
	// layer-7b set override (its PA/TA are the X/X, read before the mint)
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTokenPT)), int16(i)
	return nil
}

func (ic *instrCtx) opRecruit() error {
	ki := &ic.ki
	// OpExt ExtRecruit. The token is a fixed corpus
	// script (the lowering registered it in lc.out.Tokens); the kernel
	// finds its table CardID by that name, so no matchup remap is
	// needed and every other generated pool stays byte-identical.
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRecruit))
	return nil
}
