package kgen

import (
	"fmt"
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
	if _, ok := ic.fl["cwWord"]; ok {
		// C names the FIRST of six records, the printed colours then one
		// colour each (lower_zone.go emitCwWordAll on the OverrideAll
		// shape), so only the first has an op: intern the other five here. B
		// marks the op for the kernel's frozen colour-word colour pick
		// (static_colorwords.go ovCwRow, one row for every hit).
		for n := 1; n < 6; n++ {
			if _, err := cb.override(cb.c.overrides[in.C+n]); err != nil {
				return err
			}
		}
		ki.B = kop.Const(kop.OvCwWord)
		if err := cb.cwWordOverridesOK(i); err != nil {
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
	if _, ok := fl["cwWord"]; ok {
		// C names the FIRST of six records, the printed colours then one
		// colour each (lower_zone.go emitCwWordAnimate), so only the first
		// has an op: intern the other five here. B marks the op for the
		// kernel's frozen colour-word colour pick (C + 1 + colour).
		for n := 1; n < 6; n++ {
			if _, err := cb.override(cb.c.overrides[in.C+n]); err != nil {
				return err
			}
		}
		ki.B = kop.Const(kop.OvCwWord)
		if err := cb.cwWordOverridesOK(i); err != nil {
			return err
		}
	}
	if _, ok := fl["enterWith"]; ok {
		ki.B = kop.Const(kop.OvEnterWith)
	}
	if _, ok := fl["spellCarry"]; ok {
		// a static SpellCast rider's timed Animate of the triggering spell:
		// stamp the spell entry and apply the record as the permanent enters
		// (dirac/static_spellcarry.go)
		ki.B = kop.Const(kop.OvSpellCarry)
	}
	if cb.c.overrides[in.C].RememberAnimated {
		// Animate's RememberAnimated$ (Rise and Shine): W names the set the
		// op adds each animated permanent to at the kernel's apply tail
		// (dirac/interp_gen.go OpOverride, gorge effAnimate's
		// rememberAnimated engine/effects/combatfx.go:751).
		ki.W = kop.RefRemembered
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
	if in.Op == "DelayedTrigger" && in.CS == "DiesThisTurn" && in.B.Kind == vConst && in.B.N == 4 {
		ki.B = kop.Const(kop.DpDiesYou) // dies under the registering player's control (dirac/trig_delayeddies.go)
	} else if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 1 {
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
