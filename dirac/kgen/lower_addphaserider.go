package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// addPhaseRider lowers DB$ AddPhase with ExtraPhaseDelayedTrigger$ (gorge
// effAddPhase's rider: Moraug, Fury of Akoum's "there's an additional combat
// phase after this phase. At the beginning of that combat, untap all
// creatures you control"). The one shape that lowers: ExtraPhase$ Combat with
// AfterPhase$, FollowedBy$ and NumPhases$ omitted, behind ConditionPhases$
// Main1,Main2, so the combat splices after the main phase the grant resolves
// in and resumes at that phase's successor (kop.AddPhaseHere); the rider's
// definition SVar a raw "Mode$ Phase | Phase$ BeginCombat" trigger line
// (ValidPlayer$ You or absent: the extra combat is in the controller's own
// main phase's turn, so the active player is the controller) and its
// Execute$ (the corpus's ExtraPhaseDelayedTriggerExcute$ spelling) a body the
// latent ability lowers. The rider is an OpDelayedProgram kop.DelayExtraCombat
// registered beside the grant: the kernel arms the newest one as the walk
// consumes the grant's combat, gorge's consume-time registration
// (events.Apply ExtraPhase -1), and it fires as that combat begins.
func (lc *lowerCtx) addPhaseRider(ps *params) error {
	if lc.ab.Kind == akMana {
		return refuse("AddPhase ExtraPhaseDelayedTrigger$ in a mana ability")
	}
	if ev := ps.str("ExtraPhase"); ev != "Combat" {
		return refuse("AddPhase ExtraPhaseDelayedTrigger$ with ExtraPhase$ %q", ev)
	}
	for _, k := range []string{"AfterPhase", "FollowedBy", "NumPhases", "BeforeFirstPostCombatMainEnd"} {
		if _, ok := ps.m[k]; ok {
			return refuse("AddPhase ExtraPhaseDelayedTrigger$ with %s$", k)
		}
	}
	if strings.ReplaceAll(ps.str("ConditionPhases"), " ", "") != "Main1,Main2" {
		return refuse("AddPhase ExtraPhaseDelayedTrigger$ without ConditionPhases$ Main1,Main2")
	}
	name := ps.str("ExtraPhaseDelayedTrigger")
	trig, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok || trig.Mode != "Phase" {
		return refuse("AddPhase ExtraPhaseDelayedTrigger$ %q is no Mode$ Phase trigger line", name)
	}
	for _, k := range sortedParamKeys(trig.Params) {
		v := strings.TrimSpace(trig.Params[k])
		switch {
		case k == "Mode", k == "TriggerDescription":
		case k == "Phase" && v == "BeginCombat":
		case k == "ValidPlayer" && v == "You":
		default:
			return refuse("AddPhase ExtraPhaseDelayedTrigger$ %s$ %q", k, v)
		}
	}
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("ExtraPhaseDelayedTriggerExcute"))
	if exec == nil {
		return refuse("AddPhase ExtraPhaseDelayedTriggerExcute$ %q", ps.str("ExtraPhaseDelayedTriggerExcute"))
	}
	obj, err := lc.delayedStepRemember("beginning-of-combat", "", exec)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "AddPhase", A: cst(1), C: kop.AddPhaseCombat | kop.AddPhaseHere, Note: "queue an extra combat after this main phase (gorge effAddPhase, AfterPhase$ omitted)"})
	lat := lc.newLatent("DelayedTrigger")
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "ExtraCombat"})
	return nil
}
