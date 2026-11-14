package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DB$ AddPhase (gorge effAddPhase) and the FirstAttack$ /
// FirstCombat$ trigger gates. Only the shapes of the carriers lower; any other
// spelling refuses by name.

// addPhase lowers DB$ AddPhase as OpExt ExtAddPhase: A the NumPhases$ count,
// C the kind | splice point << 4. The (kind, splice point) pairs the kernel
// walk reproduces: a combat or an upkeep after end of combat, an end step
// after the end step, and (addPhaseFollowedBy) a combat FollowedBy$ Main2
// spliced after the main phase it resolves in, with AfterPhase$ omitted and
// its ConditionPhases$ Main1,Main2 gate lowered. Any other ConditionPhases$
// (Full Throttle, Great Train Heist) is not lowered: both cards stay refused
// for another part of the card.
func (lc *lowerCtx) addPhase(ps *params) error {
	for _, k := range []string{"ExtraPhaseDelayedTrigger", "ExtraPhaseDelayedTriggerExcute", "BeforeFirstPostCombatMainEnd"} {
		if _, ok := ps.m[k]; ok {
			return refuse("AddPhase %s$", k)
		}
	}
	var kind int
	switch ev := ps.str("ExtraPhase"); ev {
	case "Combat":
		kind = kop.AddPhaseCombat
	case "Upkeep":
		kind = kop.AddPhaseUpkeep
	case "End of Turn":
		kind = kop.AddPhaseEnd
	default:
		return refuse("AddPhase ExtraPhase$ %q", ev)
	}
	if fb, ok := ps.get("FollowedBy"); ok {
		return lc.addPhaseFollowedBy(ps, kind, fb)
	}
	var after int
	switch ap := ps.str("AfterPhase"); ap {
	case "EndCombat":
		after = int(kop.StepEndCombat)
	case "End of Turn":
		after = int(kop.StepEnd)
	default:
		// without FollowedBy$ the omitted form (splice after the step the
		// grant resolves in: Full Throttle, refused for its Effect) does not
		// lower; no other spelling does either
		return refuse("AddPhase AfterPhase$ %q", ap)
	}
	switch {
	case kind == kop.AddPhaseCombat && after == int(kop.StepEndCombat),
		kind == kop.AddPhaseUpkeep && after == int(kop.StepEndCombat),
		kind == kop.AddPhaseEnd && after == int(kop.StepEnd):
	default:
		return refuse("AddPhase ExtraPhase$ %s with AfterPhase$ %q", ps.str("ExtraPhase"), ps.str("AfterPhase"))
	}
	n, err := lc.amount(ps, "NumPhases", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "AddPhase", A: n, C: kind | after<<4, Note: "queue extra phases (gorge effAddPhase)"})
	return nil
}

// finishedEndStepsValue is Count$FinishedEndOfTurnsThisTurn (gorge
// evalCountBodyCostFinishedEndOfTurnsThisTurn): the end steps begun this
// turn, less the one being run while in the end step.
func finishedEndStepsValue() lValue {
	return lValue{Kind: vPhaseCount, N: kop.PhaseCountEndSteps}
}

// firstAttackCombat reads the FirstAttack$ True and FirstCombat$ True trigger
// gates. FirstAttack$ (gorge trigmatch firstAttackOK, AttacksThisTurn == 1 at
// the fire) is a flag on an Attacks trigger. FirstCombat$ (gorge
// trigger_match.go, CombatsThisTurn == 1 at the fire) is a fire-only
// condition on an Attacks or AttackersDeclared trigger (Karlach, Fury of
// Avernus: both fire in the declare-attackers step of the combat the count
// names); it reports that, for the caller's CheckOnTriggerOnly.
func (lc *lowerCtx) firstAttackCombat(tp *params, tr *lTrigger, conds *[]lCond, mode string) (bool, error) {
	if v, ok := tp.get("FirstAttack"); ok {
		if strings.TrimSpace(v) != "True" || mode != "Attacks" || tr.Event != "Attacks" {
			return false, refuse("FirstAttack$ %q on a %s trigger", v, mode)
		}
		tr.FirstAttack = true
	}
	if v, ok := tp.get("FirstCombat"); ok {
		if strings.TrimSpace(v) != "True" || mode != "Attacks" && mode != "AttackersDeclared" {
			return false, refuse("FirstCombat$ %q on a %s trigger", v, mode)
		}
		*conds = append(*conds, lCond{Kind: cValue, A: lValue{Kind: vPhaseCount, N: kop.PhaseCountCombats}, Op: opEQ, B: cst(1)})
		return true, nil
	}
	return false, nil
}

// addPhaseFollowedBy lowers the one FollowedBy$ shape: an extra combat spliced
// after the main phase the grant resolves in (AfterPhase$ omitted) and followed
// by an additional main phase, behind ConditionPhases$ Main1,Main2 (the
// Aggravated Assault family; Relentless Assault). The gate is lowered with the
// condition and is what keeps the splice point a main phase.
func (lc *lowerCtx) addPhaseFollowedBy(ps *params, kind int, fb string) error {
	if strings.TrimSpace(fb) != "Main2" || kind != kop.AddPhaseCombat {
		return refuse("AddPhase FollowedBy$ %q with ExtraPhase$ %s", fb, ps.str("ExtraPhase"))
	}
	if _, ok := ps.get("AfterPhase"); ok {
		return refuse("AddPhase FollowedBy$ with AfterPhase$ %q", ps.str("AfterPhase"))
	}
	if strings.ReplaceAll(ps.str("ConditionPhases"), " ", "") != "Main1,Main2" {
		return refuse("AddPhase FollowedBy$ without ConditionPhases$ Main1,Main2")
	}
	if _, ok := ps.m["NumPhases"]; ok {
		return refuse("AddPhase FollowedBy$ with NumPhases$")
	}
	lc.emit(lInstr{Op: "AddPhase", A: cst(1), C: kop.AddPhaseCombatMain, Note: "queue an extra combat and an additional main phase (gorge effAddPhase FollowedBy$)"})
	return nil
}
