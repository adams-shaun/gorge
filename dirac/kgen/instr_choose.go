package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for choices: players, colours, types, names and numbers (instr.go).
func init() {
	instrOp("ChooseColorRes", (*instrCtx).opChooseColorRes)
	instrOp("Vote", (*instrCtx).opVote)
	instrOp("ChoosePlayerRes", (*instrCtx).opChoosePlayerRes)
	instrOp("RollDie", (*instrCtx).opRollDie)
	instrOp("ChoosePlayer", (*instrCtx).opChoosePlayer)
	instrOp("ChooseColor", (*instrCtx).opChooseColor)
	instrOp("ChooseType", (*instrCtx).opChooseType)
	instrOp("ChooseName", (*instrCtx).opChooseName)
	instrOp("ChooseCardEnter", (*instrCtx).opChooseCardEnter)
	instrOp("EnterCtrPick", (*instrCtx).opEnterCtrPick)
	instrOp("ChoosePlayerEnter", (*instrCtx).opChoosePlayerEnter)
	instrOp("ChoosePlayerMid", (*instrCtx).opChoosePlayerMid)
	instrOp("ChoosePlayerEach", (*instrCtx).opChoosePlayerEach)
	instrOp("ChooseNumber", (*instrCtx).opChooseNumber)
	instrOp("ChooseNumberMid", (*instrCtx).opChooseNumberMid)
	instrOp("NameCard", (*instrCtx).opNameCard)
	instrOp("ClearNamedCard", (*instrCtx).opClearNamedCard)
}

func (ic *instrCtx) opChooseColorRes() error {
	ki := &ic.ki
	// (Skrelv) OpExt ExtChooseColorRes; C = 1 the no-ask white answer of
	// OrColors$ (lower_choosecolor.go chooseColorOrColors)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseColorRes)), int16(ic.in.C)
	return nil
}

func (ic *instrCtx) opVote() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtVote, F the card ballot (battlefield permanents), C bit 0
	// exile the most-voted; the fixed list packs mode bit 1, the choice
	// count in bits 2..4 and the tie branch in bit 5 (lower_vote.go,
	// dirac/act_vote.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtVote)), int16(in.C)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opChoosePlayerRes() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtChoosePlayerRes, C bit 0 the Player.!IsRemembered pool
	// (act_chooseplayerres.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChoosePlayerRes)), int16(in.C&(kop.ChoosePlayerFresh|kop.ChoosePlayerForget))
	return nil
}

func (ic *instrCtx) opRollDie() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpRollDie, int16(in.C)
	return nil
}

func (ic *instrCtx) opChoosePlayer() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpChoosePlayer
	switch in.CS {
	case "Random":
		ki.C = kop.ChPlRandom
	case "RandomOpp":
		ki.C = kop.ChPlRandom | kop.ChPlPlain
	default:
		return fmt.Errorf("ChoosePlayer build %q", in.CS)
	}
	return nil
}

func (ic *instrCtx) opChooseColor() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpChooseColor, int16(in.C) // in.C: the Exclude$ colours (W U B R G = bits 0..4)
	return nil
}

func (ic *instrCtx) opChooseType() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpChooseType, int16(in.C) // C: a card-type category's offered types (lower_choosetype.go)
	if in.C == 0 && in.B.N == kop.ChooseTypeBasicLand {
		// the answer is a pool subtype index (basicLandAnswer): all five basic
		// land types must be in the pool table
		if _, err := ic.cb.subIndexes(basicLandChoice); err != nil {
			return err
		}
	}
	// B = kop.ChooseTypeLand interns nothing: the answer is a pool subtype index
	// (dirac landValue) over the land types the pool table already holds, and
	// one it lacks answers 255, which a ChosenType read matches to no land on
	// either surface (no card of the pool has the type).
	if in.B.N == kop.ChooseTypeRandomDeck {
		// the no-ask fallback records "Human" when the resolving controller
		// controls no creature (gorge creatureTypeFallback): the word must be
		// in the pool table, or the kernel could not name the answer
		if _, err := ic.cb.subIndexes([]string{"Human"}); err != nil {
			return fmt.Errorf("ChooseType random-deck fallback: %w", err)
		}
	}
	return nil
}

func (ic *instrCtx) opChooseName() error {
	ki := &ic.ki
	in := &ic.in
	buildChooseName(ki, in.B.N) // an as-enters card name (lower_choosename.go)
	return nil
}

func (ic *instrCtx) opChooseCardEnter() error {
	ki := &ic.ki
	buildChooseCardEnter(ki) // an as-enters object choice (lower_choosecardenter.go)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opChoosePlayerEnter() error {
	ki := &ic.ki
	// an as-enters opponent (lower_chooseplayerenter.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChoosePlayer))
	return nil
}

func (ic *instrCtx) opChoosePlayerMid() error {
	ki := &ic.ki
	// a mid-resolution opponent choice (lower_chooseplayermid.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChoosePlayerMid)), int16(ic.in.C&kop.ChoosePlayerMidHigh)
	return nil
}

func (ic *instrCtx) opChoosePlayerEach() error {
	ki := &ic.ki
	// the repeated player's opponent choice (lower_chooseplayer.go
	// choosePlayerEach): no operands, the loop's binding is the chooser
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChoosePlayerEach))
	return nil
}

func (ic *instrCtx) opChooseNumber() error {
	ki := &ic.ki
	in := &ic.in
	// an as-enters number from B to C (lower_choosenumber.go)
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseNumber)), kop.Const(in.B.N), int16(in.C)
	return nil
}

func (ic *instrCtx) opChooseNumberMid() error {
	ki := &ic.ki
	in := &ic.in
	// a resolution-time number (lower_choosenumber.go): B the Max$
	// value, C the Min$ floor plus the has-Max / Random flags
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseNumberMid)), int16(in.C)
	return nil
}

func (ic *instrCtx) opEnterCtrPick() error {
	return buildEnterCtrPick(&ic.ki, ic.in.CS) // an as-enters pair of counter kinds (lower_enterctrpick.go)
}

func (ic *instrCtx) opNameCard() error {
	// a mid-resolution card name (lower_namecard.go)
	buildNameCard(&ic.ki, ic.in.B.N)
	return nil
}

func (ic *instrCtx) opClearNamedCard() error {
	// gorge `DB$ Cleanup | ClearNamedCard$ True` (lower_namecard.go)
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtClearNamedCard))
	return nil
}
