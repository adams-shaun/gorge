package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for free casts, play permissions and game designations (instr.go).
func init() {
	instrOp("GiveGift", (*instrCtx).opGiveGift)
	instrOp("TakeInitiative", (*instrCtx).opTakeInitiative)
	instrOp("Venture", (*instrCtx).opVenture)
	instrOp("MayPlayGrave", (*instrCtx).opMayPlayGrave)
	instrOp("BecomeMonarch", (*instrCtx).opBecomeMonarch)
	instrOp("RingTemptsYou", (*instrCtx).opRingTemptsYou)
	instrOp("TurnRepl", (*instrCtx).opTurnRepl)
	instrOp("CastFree", (*instrCtx).opCastFree)
	instrOp("MayPlay", (*instrCtx).opMayPlay)
}

func (ic *instrCtx) opGiveGift() error {
	ki := &ic.ki
	ki.Op = kop.OpGiveGift
	return nil
}

func (ic *instrCtx) opTakeInitiative() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpTakeInitiative
	idx, ok := cb.pt.tokenIdx[in.CS]
	if !ok {
		return fmt.Errorf("dungeon %q", in.CS)
	}
	ki.C = int16(idx)
	return nil
}

func (ic *instrCtx) opVenture() error {
	ki := &ic.ki
	// OpExt ExtVenture, Obj the player (dirac/act_venture.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtVenture))
	return nil
}

func (ic *instrCtx) opMayPlayGrave() error {
	ki := &ic.ki
	// (Case of the Uneaten Feast) OpExt ExtMayPlayGrave
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMayPlayGrave))
	if err := ic.objFilter(kop.ZoneGraveyard); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opBecomeMonarch() error {
	ki := &ic.ki
	// OpExt ExtBecomeMonarch, Obj the player (dirac/act_monarch.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBecomeMonarch))
	return nil
}

func (ic *instrCtx) opRingTemptsYou() error {
	ki := &ic.ki
	// OpExt ExtRingTemptsYou, Obj the player (dirac/act_ring.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRingTemptsYou))
	return nil
}

func (ic *instrCtx) opTurnRepl() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtTurnRepl, C the kop.TurnRepl* kind (dirac/repl_turnrepl.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTurnRepl)), int16(in.C)
	return nil
}

func (ic *instrCtx) opCastFree() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	// C: zone | AmountOne<<3 | ForgetPlayed<<4 | ReplaceGraveyard<<5 | CopyCard<<6 | Repeat<<7 | Paid<<8 | UpTo<<9 | SpellMV<<10 | Mandatory<<11 | Lands<<12
	// (Repeat: A is the copy count, AllowRepeats$ with Amount$ X)
	// Obj: RefRemembered (P1), a target slot (P2), or
	// RefNone with F (P3/P4 filter population).
	ki.Op = kop.OpCastFree
	ki.C = int16(zoneMap[in.C])
	if _, ok := fl["One"]; ok {
		ki.C |= 1 << 3
	}
	if _, ok := fl["Forget"]; ok {
		ki.C |= 1 << 4
	}
	if _, ok := fl["ReplaceGY"]; ok {
		ki.C |= 1 << 5
	}
	if _, ok := fl["Copy"]; ok {
		ki.C |= 1 << 6 // Play CopyCard$ True
	}
	if _, ok := fl["Repeat"]; ok {
		ki.C |= 1 << 7 // AllowRepeats$ True with Amount$ X: A copies offered
	}
	if _, ok := fl["Paid"]; ok {
		ki.C |= 1 << 8 // the copy pays its mana cost (an exile-granted ability's Play)
	}
	if _, ok := fl["UpTo"]; ok {
		ki.C |= 1 << 9 // a literal Amount$ N: the ask closes after A picks
	}
	if _, ok := fl["SpellMV"]; ok {
		ki.C |= 1 << 10 // the ValidSA$ mana-value bound reads the spell: a split card counts its front half
	}
	if _, ok := fl["Mandatory"]; ok {
		ki.C |= 1 << 11 // no Optional$ True: the offer has no Done while a candidate is left
	}
	if _, ok := fl["Lands"]; ok {
		ki.C |= 1 << 12 // the population may hold lands, offered as land plays (dirac/cast_freeland.go)
	}
	if err := ic.objFilter(zoneMap[in.C]); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opMayPlay() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpMayPlay, int16(zoneMap[in.C])
	if in.Follows {
		ki.C |= kop.MayPlayFollows // the record follows its card (kop/operands.go)
	}
	switch in.CS {
	case "EOT":
		ki.A = kop.Const(kop.DurEOT)
	case "EndOfYourNextTurn":
		ki.A = kop.Const(kop.DurYourNextTurn)
	case "UntilYourNextTurn":
		ki.A = kop.Const(kop.DurUntilYourNextTurn)
	case "WhileGraveForget":
		ki.A = kop.Const(kop.DurWhileGraveForget) // the record survives a return to the graveyard; a move to exile ends it
	case "WhileExiled":
		ki.A = kop.Const(kop.DurPermanent) // for as long as it remains exiled
		if in.F != nil {
			// the remembered cards the grant admits (Vivien, Champion of
			// the Wilds: Creature.IsRemembered), matched as the record is made
			if err := ic.objFilter(kop.ZoneExile); err != nil {
				return err
			}
		}
	case "WhileSrc":
		ki.A = kop.Const(kop.DurWhileSrcMay) // for as long as the Effect's source is on the battlefield
	case "WhileValid":
		ki.A = kop.Const(kop.DurWhileValid) // while exiled, and while the controller controls a match of in.F
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	default:
		return fmt.Errorf("may-play duration %q", in.CS)
	}
	return nil
}
