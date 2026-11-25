package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for single-object zone moves and library placement (instr.go).
func init() {
	instrOp("Exile", (*instrCtx).opExile)
	instrOp("Bounce", (*instrCtx).opBounce)
	instrOp("ToLibrary", (*instrCtx).opToLibrary)
	instrOp("ToGraveyard", (*instrCtx).opToGraveyard)
	instrOp("MayShuffleMove", (*instrCtx).opMayShuffleMove)
	instrOp("Return", (*instrCtx).opReturn)
	instrOp("ExileUntilLeaves", (*instrCtx).opExileUntilLeaves)
	instrOp("Unearth", (*instrCtx).opUnearth)
	instrOp("LeaveExile", (*instrCtx).opLeaveExile)
	instrOp("LibSweep", (*instrCtx).opLibSweep)
	instrOp("ExileExMark", (*instrCtx).opExileExMark)
	instrOp("SpellToLib", (*instrCtx).opSpellToLib)
	instrOp("LibPlace", (*instrCtx).opLibPlace)
	instrOp("StackExileMark", (*instrCtx).opStackExileMark)
	instrOp("ExileLandChoose", (*instrCtx).opExileLandChoose)
	instrOp("ExileEmbRem", (*instrCtx).opExileEmbRem)
	instrOp("ToLibraryPair", (*instrCtx).opToLibraryPair)
	instrOp("Shuffle", (*instrCtx).opShuffle)
	instrOp("Rearrange", (*instrCtx).opRearrange)
	instrOp("DelayLeaveReturn", (*instrCtx).opDelayLeaveReturn)
}

func (ic *instrCtx) opExile() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	ki.Op = kop.OpExile
	if m := ic.moveFlags(); m != 0 || fl["counters"] != "" {
		ki.Op, ki.C = kop.OpExileEx, int16(m)
		if err := ic.counters(); err != nil {
			return err
		}
	}
	if fl["from"] == "Battlefield" {
		// Origin$ Battlefield on any referent: only one still on the
		// battlefield moves (gorge effChangeZone's Origin$ precondition).
		// A triggering permanent that left before the exile resolved is a
		// new object this ability cannot find (CR 400.7, 608.2b); an
		// Enchanted/Equipped referent reads as LastBearer off the
		// battlefield (CR 608.2h) and does not move either.
		ki.C |= kop.MvFromBattlefield
	}
	if (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
		ki.C |= kop.MvFromGraveyard
	}
	return nil
}

func (ic *instrCtx) opBounce() error {
	ki := &ic.ki
	fl := ic.fl
	ki.Op = kop.OpBounce
	// RememberChanged$: MvRemember; RememberLKI$: MvRemember|MvLKI
	// (zone_rememberparams.go#bounceRemember reads those two bits and
	// MvForgetChanged).
	if m := ic.moveFlags(); m != 0 {
		if m&^(kop.MvRemember|kop.MvLKI|kop.MvForgetChanged) != 0 {
			return fmt.Errorf("bounce move flags %#x", m)
		}
		ki.C = int16(m)
	}
	if fl["from"] == "Library" {
		// Origin$ Library: a library card only, then the owner's shuffle
		ki.C |= kop.BncFromLibrary
		if _, ok := fl["ShuffleOwed"]; ok {
			ki.C |= kop.BncShuffleOwed
		}
	}
	if _, ok := fl["PermOrSpell"]; ok {
		ki.C |= kop.MvPermOrSpell
	}
	if _, ok := fl["DelayedMany"]; ok {
		ki.C = kop.BncDelayedMany // the delayed many-object return (dirac/trig_delayedmany.go)
	}
	return nil
}

func (ic *instrCtx) opToLibrary() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	ki.Op, ki.C = kop.OpToLibrary, int16(in.C)
	if (in.C == 0 || in.C == 1) && (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
		ki.C = kop.LibTopGrave + int16(in.C) // dirac/zone_tolibrarygrave.go
	}
	if _, rem := fl["Remember"]; rem && in.C == 1 && in.Obj.Kind == rDelayedRemembered && fl["from"] == "Battlefield" {
		ki.C = kop.LibBottomRememberBF // dirac/zone_tolibrarybfremember.go
	}
	if _, rem := fl["Remember"]; rem && in.C == 1 && in.Obj.Kind == rTarget && fl["from"] == "Exile" {
		ki.C = kop.LibBottomRememberExile // dirac/zone_tolibrarybfremember.go
	}
	if _, rem := fl["Remember"]; rem && in.C == 0 && in.Obj.Kind == rSelf && fl["from"] == "Battlefield" {
		ki.C = kop.LibTopRememberBF // dirac/zone_tolibrarybfremember.go
	}
	if _, rem := fl["Remember"]; rem && fl["from"] == "Graveyard" {
		// RememberChanged$ on an Origin$ Graveyard move (a target, or the
		// trigger object itself): the card that reached the library joins
		// the remembered set the following read sees.
		if in.C == 0 {
			ki.C = kop.LibTopRememberGrave // dirac/zone_tolibraryremember.go
		} else if in.C == 1 {
			ki.C = kop.LibBottomRememberGrave // dirac/zone_tolibraryremember.go
		}
	}
	if _, ok := fl["TopShuffle"]; ok {
		ki.C = kop.LibTopShuffle
		if (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
			ki.C = kop.LibTopShuffleGrave
		}
	}
	if _, ok := fl["TopShuffleSelf"]; ok {
		ki.C = kop.LibTopShuffleSelf
	}
	if _, ok := fl["BottomShuffle"]; ok {
		ki.C = kop.LibBottomShuffle
	}
	if in.C == 3 {
		ki.Op, ki.C = kop.OpToLibraryChoice, 0
	}
	if in.C == 4 {
		ki.Op, ki.C = kop.OpToLibraryChoice, kop.LibChoiceSecond
	}
	if in.C == libChoiceCtrlGrave { // lower_destalt.go
		ki.Op, ki.C = kop.OpToLibraryChoice, kop.LibChoiceCtrlGrave
	}
	return nil
}

func (ic *instrCtx) opToGraveyard() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtToGraveyard))
	return nil
}

func (ic *instrCtx) opMayShuffleMove() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpMayShuffleMove, int16(in.C)
	return nil
}

func (ic *instrCtx) opReturn() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	var err error
	ki.Op, ki.C = kop.OpReturn, int16(zoneMap[in.C])
	_, underOpp := fl["UnderOpponent"]
	_, attachTo := fl["AttachTo"]
	if m := ic.moveFlags(); m != 0 || fl["counters"] != "" || fl["from"] == "Library" || underOpp || attachTo {
		ki.Op = kop.OpReturnEx
		ki.C = int16(int(zoneMap[in.C]) | m<<3)
		if err := ic.counters(); err != nil {
			return err
		}
	}
	if _, any := fl["FromAny"]; any && ki.Op == kop.OpReturnEx {
		// Origin$ All on a remembered card: a library card moves too
		// (kop.MvEntryRiders' bit, OpReturnEx's C sign bit, set through an int
		// so the shifted flag wraps into int16)
		bit := kop.MvEntryRiders
		ki.C |= int16(bit << 3)
	}
	if underOpp {
		// GainControl$ ChosenPlayer (one opponent): B.N rides beside the library bits
		if fl["counters"] != "" || fl["from"] == "Library" {
			return fmt.Errorf("Return under the opponent with counters or from the library")
		}
		ki.B = kop.Const(kop.RetUnderOpponent)
	}
	if attachTo {
		// AttachedTo$: the bearer's Ref rides
		// B bits 8..10 beside RetAttachTo (kop.RetAttachShift)
		if fl["counters"] != "" || underOpp || fl["from"] == "Library" {
			return fmt.Errorf("Return attached with counters, UnderOpponent or from the library")
		}
		var r kop.Ref
		if in.F != nil {
			// a filter bearer (One Last Job): the controller
			// chooses among the legal permanents it matches, so F rides
			// the instruction and the Ref stays zero
			if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
				return err
			}
		} else if r, err = cb.ref(in.Obj2); err != nil {
			return err
		}
		if r > 7 {
			return fmt.Errorf("attach bearer ref %d", r)
		}
		ki.B = kop.Const(kop.RetAttachTo | int(r)<<kop.RetAttachShift)
	}
	if ki.Op == kop.OpReturn {
		ki.B = kop.Const(originMask(fl["from"]))
		if _, ok := fl["DelayedMany"]; ok {
			ki.B = kop.Const(originMask(fl["from"]) | kop.RetDelayedMany) // the delayed many-object return (dirac/trig_delayedmanyreturn.go)
		}
	}
	if fl["from"] == "Library" {
		// Origin$ Library: the move takes a library card only, then its
		// owner's library is shuffled (the kernel otherwise skips a library
		// object)
		if fl["counters"] != "" {
			return fmt.Errorf("Return from the library with counters")
		}
		if _, ok := fl["ForgetOther"]; ok {
			return fmt.Errorf("Return from the library with ForgetOtherRemembered$")
		}
		if _, ok := fl["AlsoSelf"]; ok {
			return fmt.Errorf("Return from the library with ThisDefinedAndTgts$")
		}
		ki.B = kop.Const(kop.RetFromLibrary)
		if _, ok := fl["ShuffleOwed"]; ok {
			ki.B = kop.Const(kop.RetFromLibrary | kop.RetShuffleOwed)
		}
	}
	return nil
}

func (ic *instrCtx) opExileUntilLeaves() error {
	ki := &ic.ki
	ki.Op = kop.OpExileUntilLeaves
	if ic.moveFlags()&kop.MvRemember != 0 {
		ki.C = kop.MvRemember // RememberChanged$ (lower_remembered_writers.go)
	}
	return nil
}

func (ic *instrCtx) opLeaveExile() error {
	ki := &ic.ki
	// OpExt ExtLeaveExile over Obj, the object the move before it returned
	// (lower_zone.go's LeaveBattlefield$ Exile, dirac/zone_leaveexile.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLeaveExile))
	return nil
}

func (ic *instrCtx) opUnearth() error {
	ki := &ic.ki
	ki.Op = kop.OpUnearth
	return nil
}

func (ic *instrCtx) opLibSweep() error {
	ki := &ic.ki
	// (Teferi, Temporal Pilgrim) OpExt ExtLibSweep, F the swept permanents
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLibSweep))
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opExileExMark() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtExileExMark over the target slot Obj;
	// C the Fight's fighter ref (RefNone for a Pump)
	r, err := cb.ref(in.Obj2)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExileExMark)), int16(r)
	return nil
}

func (ic *instrCtx) opSpellToLib() error {
	ki := &ic.ki
	// OpExt ExtSpellToLib: the resolving spell to its owner's library, shuffled
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtSpellToLib))
	return nil
}

func (ic *instrCtx) opLibPlace() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtLibPlace, C the arm (top / bottom) or the ask (lower_libplace.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtLibPlace)), int16(in.C)
	return nil
}

func (ic *instrCtx) opStackExileMark() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtStackExileMark, C the kop.StackExile* bits (repl_stackexile.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtStackExileMark)), int16(in.C)
	return nil
}

func (ic *instrCtx) opExileLandChoose() error {
	ki := &ic.ki
	// (Ensign Pavel Chekov) OpExt ExtExileLandChoose
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExileLandChoose))
	return nil
}

func (ic *instrCtx) opExileEmbRem() error {
	ki := &ic.ki
	// OpExt ExtExileEmbRem over the firing emblem's remembered set (lower_effectremember.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExileEmbRem))
	return nil
}

func (ic *instrCtx) opToLibraryPair() error {
	ki := &ic.ki
	ki.Op = kop.OpToLibraryPair
	return nil
}

func (ic *instrCtx) opShuffle() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpShuffle, int16(in.C) // in.C 1: every remembered owner (lower_shuffleremowners.go)
	return nil
}

func (ic *instrCtx) opRearrange() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpRearrange, int16(in.C)
	return nil
}

func (ic *instrCtx) opDelayLeaveReturn() error {
	ki := &ic.ki
	// OpDelayedTrigger C = the kernel's internal leave-return act
	// (dirac/trig_leavereturn.go opLeaveReturn)
	ki.Op, ki.C = kop.OpDelayedTrigger, kernelOpLeaveReturn
	return nil
}
