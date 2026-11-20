package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for overrides, emblems, activation locks and delayed triggers (instr.go).
func init() {
	instrOp("LockActs", (*instrCtx).opLockActs)
	instrOp("CreateEmblem", (*instrCtx).opCreateEmblem)
	instrOp("OverrideAll", (*instrCtx).opOverrideAll)
	instrOp("Override", (*instrCtx).opOverride)
	instrOp("DelayedTrigger", (*instrCtx).opDelayedTrigger)
	instrOp("ReflexiveTrigger", (*instrCtx).opDelayedTrigger)
}

func (ic *instrCtx) opLockActs() error {
	ki := &ic.ki
	// (Braided Net) OpExt ExtLockActs on the target
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLockActs))
	return nil
}

func (ic *instrCtx) opCreateEmblem() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpCreateEmblem, int16(in.C)
	return nil
}

func (ic *instrCtx) opOverrideAll() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpOverrideAll
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.C = int16(i)
	switch in.CS {
	case "EOT":
		ki.A = kop.Const(kop.DurEOT)
	case "Permanent":
		ki.A = kop.Const(kop.DurPermanent)
	case "UntilYourNextTurn":
		ki.A = kop.Const(kop.DurUntilYourNextTurn)
	default:
		return fmt.Errorf("override duration %q", in.CS)
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opOverride() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	ki.Op = kop.OpOverride
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.C = int16(i)
	if _, ok := fl["chosenColor"]; ok {
		// C names the FIRST of five records
		// differing only in SetColors (W, U, B, R, G in order); only
		// the first has an op, so intern the other four here. B marks
		// the op for the kernel's C + res.chosen - 1 record pick.
		for n := 1; n < 5; n++ {
			if _, err := cb.override(cb.c.overrides[in.C+n]); err != nil {
				return err
			}
		}
		ki.B = kop.Const(kop.OvChosenColor)
		if err := cb.chosenColorOverridesOK(i); err != nil {
			return err
		}
	}
	if _, ok := fl["chosenType"]; ok {
		// C names the FIRST of five records differing only in
		// AddSub, one basic land type each (lower_choosetype.go
		// emitChosenLandTypeAnimate); B marks the op for the
		// kernel's pick of the chosen type's record.
		for n := 1; n < 5; n++ {
			if _, err := cb.override(cb.c.overrides[in.C+n]); err != nil {
				return err
			}
		}
		ki.B = kop.Const(kop.OvChosenType)
		if err := cb.chosenTypeOverridesOK(i); err != nil {
			return err
		}
	}
	if _, ok := fl["enterWith"]; ok {
		ki.B = kop.Const(kop.OvEnterWith)
	}
	if nv, ok := fl["protChoice"]; ok {
		// C names the FIRST of N records differing only in ProtFrom, one
		// protection quality each in gorge's offer order
		// (lower_protection.go emitProtectionChoice); B marks the op for
		// the kernel's pick of the answered option's record.
		n, err := strconv.Atoi(nv)
		if err != nil || n < 2 {
			return fmt.Errorf("protection choice family %q", nv)
		}
		for m := 1; m < n; m++ {
			if _, err := cb.override(cb.c.overrides[in.C+m]); err != nil {
				return err
			}
		}
		ki.B = kop.Const(kop.OvProtChoice)
		if err := cb.protChoiceOverridesOK(i, n); err != nil {
			return err
		}
	}
	switch strings.SplitN(in.CS, "|", 2)[0] {
	case "EOT":
		ki.A = kop.Const(kop.DurEOT)
	case "Permanent":
		ki.A = kop.Const(kop.DurPermanent)
	case "UntilYourNextTurn":
		ki.A = kop.Const(kop.DurUntilYourNextTurn)
	case "UntilYourNextUpkeep":
		ki.A = kop.Const(kop.DurUntilYourNextUpkeep)
	case "WhileHost":
		ki.A = kop.Const(kop.DurPermanent) // Override.WhileHost bounds it by the host
	default:
		return fmt.Errorf("override duration %q", in.CS)
	}
	return nil
}

func (ic *instrCtx) opDelayedTrigger() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpDelayedProgram
	if in.Op == "ReflexiveTrigger" {
		ki.Op = kop.OpReflexiveTrigger
	} else {
		switch in.CS {
		case "NextEndStep":
		case "YourNextEndStep":
			ki.A = kop.Const(kop.DelayYourEndStep) // the controller's next end step (dirac/trig_delayedtiming.go)
		case "NextUpkeep":
			ki.A = kop.Const(kop.DelayUpkeep)
		case "YourNextUpkeep":
			ki.A = kop.Const(kop.DelayYourUpkeep)
		case "NextEndCombat":
			ki.A = kop.Const(kop.DelayEndCombat)
		case "NextUpkeepOfRemPlayer":
			ki.A = kop.Const(kop.DelayRemUpkeep) // dirac/trig_delayremupkeep.go
		case "NextUpkeepOfPlayer":
			ki.A = kop.Const(kop.DelayPlayerUpkeep) // dirac/trig_delayplayerupkeep.go
		case "NextBeginCombat":
			ki.A = kop.Const(kop.DelayBeginCombat) // dirac/trig_delaybegincombat.go
		case "NextSpellCast":
			ki.A = kop.Const(kop.DelaySpellCast)
		case "NextSpellCastUncounter":
			ki.A = kop.Const(kop.DelaySpellUncounter) // dirac/repl_uncounterable.go
		case "DiesThisTurn":
			ki.A = kop.Const(kop.DelayDiesThisTurn) // dirac/trig_delayeddies.go
		default:
			return fmt.Errorf("delayed trigger %q", in.CS)
		}
	}
	if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 1 {
		ki.B = kop.Const(kop.DpGroup) // Mobilize's group program (dirac/kw_mobilize.go)
	}
	if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 3 {
		ki.B = kop.Const(kop.DpMany) // a many-object remembered set (dirac/trig_delayedmany.go)
	}
	if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 2 {
		ki.B = kop.Const(kop.DpTrack) // Decayed's incarnation-tracked promise (dirac/kw_decayed.go)
	}
	j, ok := cb.abMap[in.C]
	if !ok {
		return fmt.Errorf("latent ability %d not emitted", in.C)
	}
	ki.C = int16(j)
	return nil
}
