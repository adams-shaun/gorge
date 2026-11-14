package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// aiHintSVars reports whether an AddSVar$ value names only SVars of this
// face in Forge's "SVar:<Name>:<Value>" grant shape: gorge's layer walk
// grants that variable to the affected object (parseSVarGrant), and its only
// reader, Engine.GrantedSVar, has no rules caller -- the corpus's granted
// variables are AI hints (HasAttackEffect, MustBeBlocked, AITapDown, SacMe).
func (lc *lowerCtx) aiHintSVars(v string) bool {
	names := strings.FieldsFunc(v, func(r rune) bool { return r == '&' || r == ',' })
	if len(names) == 0 {
		return false
	}
	for _, n := range names {
		if !strings.HasPrefix(lc.face.SVars[strings.TrimSpace(n)], "SVar:") {
			return false
		}
	}
	return true
}

// bareCondition lowers a node's bare Condition$ (gorge conditionBareMet):
// Kicked reads the resolving spell's cast provenance (the kernel's
// CondKicked reads the resolving entry, so only a spell's own chain
// lowers); Threshold and Delirium read the resolving controller's
// graveyard; Metalcraft counts the controller's artifacts. Any other spelling, or one beside a ConditionPresent$/
// ConditionCheckSVar$ gate, refuses.
func (lc *lowerCtx) bareCondition(ps *params, c string) (lCond, error) {
	for _, k := range []string{"ConditionPresent", "ConditionCheckSVar", "ConditionDefined", "ConditionCompare", "ConditionSVarCompare"} {
		if _, ok := ps.m[k]; ok {
			return lCond{}, refuse("Condition$ %q beside %s$", c, k)
		}
	}
	switch c {
	case "Kicked", "Bargain":
		if c == "Bargain" && !lc.out.Bargain {
			return lCond{}, refuse("Condition$ Bargain without Bargain")
		}
		if lc.ab == nil || lc.ab.Kind != akSpell {
			return lCond{}, refuse("Condition$ %s outside a spell", c)
		}
		return lCond{Kind: cKicked}, nil
	case "OptionalCost":
		return lc.optCostCond(ps) // lower_optcost.go
	case "Threshold":
		return lCond{Kind: cThreshold}, nil
	case "Delirium":
		return deliriumCond(), nil
	case "Metalcraft":
		// gorge metalcraftHolds: three or more artifacts the resolving
		// controller controls (CR 207.2c)
		f, err := lc.filter("Artifact.YouCtrl")
		if err != nil {
			return lCond{}, err
		}
		return lCond{Kind: cValue, A: lValue{Kind: vCount, F: f}, Op: opGE, B: cst(3)}, nil
	}
	return lCond{}, refuse("Condition$ %q", c)
}

// conditionKeysBesides returns ps's Condition*$ keys other than
// ConditionDescription$ and the allowed ones, sorted. A refusal that names
// the first unlisted key iterates the sorted list, never the map.
func conditionKeysBesides(ps *params, allowed ...string) []string {
	skip := map[string]bool{"ConditionDescription": true}
	for _, k := range allowed {
		skip[k] = true
	}
	var out []string
	for k := range ps.m {
		if strings.HasPrefix(k, "Condition") && !skip[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// phaseStepBits maps gorge's step numbering (engine/state Step) onto the
// kernel's (kop.Step). The two orders are equal through declare blockers;
// the kernel then models first-strike and regular combat damage as separate
// steps where gorge has one combat-damage step, so every later gorge step
// shifts by one. The zero Step is StepUntap, so a zero entry means "not
// mapped" and is never a valid mask. Read-only; never ranged where order
// could reach an event.
var phaseStepBits = [state.StepCleanup + 1]int{
	state.StepUntap:            1 << kop.StepUntap,
	state.StepUpkeep:           1 << kop.StepUpkeep,
	state.StepDraw:             1 << kop.StepDraw,
	state.StepMain1:            1 << kop.StepMain1,
	state.StepBeginCombat:      1 << kop.StepBeginCombat,
	state.StepDeclareAttackers: 1 << kop.StepAttackers,
	state.StepDeclareBlockers:  1 << kop.StepBlockers,
	// gorge's one combat-damage step is both of the kernel's damage steps
	state.StepCombatDamage: 1<<kop.StepFirstStrikeDamage | 1<<kop.StepDamage,
	state.StepEndCombat:    1 << kop.StepEndCombat,
	state.StepMain2:        1 << kop.StepMain2,
	state.StepEnd:          1 << kop.StepEnd,
	state.StepCleanup:      1 << kop.StepCleanup,
}

// phasesCondition lowers ConditionPhases$ <list> (gorge conditions.go's
// phase gate) to a vStepIn test. The list parses through the ONE shared
// phase-name parser, state.ParsePhases: an unknown name, an empty resolved
// set, or a step the kernel does not model refuses.
func (lc *lowerCtx) phasesCondition(spec string) (lCond, error) {
	return phasesCond("ConditionPhases", spec)
}

// phasesCond is phasesCondition for the parameter named key (an activation
// window's ActivationPhases$ reads the same parser).
func phasesCond(key, spec string) (lCond, error) {
	set, unknown := state.ParsePhases(spec)
	if len(unknown) > 0 {
		return lCond{}, refuse("%s$ %q: unknown phase %q", key, spec, unknown[0])
	}
	if set.Empty() {
		return lCond{}, refuse("%s$ %q names no step", key, spec)
	}
	var mask int
	for _, st := range set.Steps() {
		if int(st) >= len(phaseStepBits) || phaseStepBits[st] == 0 {
			return lCond{}, refuse("%s$ step %d", key, st)
		}
		mask |= phaseStepBits[st]
	}
	return lCond{Kind: cValue, A: lValue{Kind: vStepIn, N: mask}, Op: opEQ, B: cst(1)}, nil
}

// playerTurnCondition lowers a lone ConditionPlayerTurn$ True|False (gorge
// conditions.go: g.Active compared with the resolving controller), or that
// turn test conjoined with ConditionPhases$ (Great Train Heist: the extra
// combat only in its controller's combat phase), and with a
// ConditionDefined$+ConditionPresent$ group gate (the Addendum family:
// Tail Swipe's ConditionDefined$ Self | ConditionPresent$ Card.wasCast).
// Beside any other Condition*$ gate it refuses.
func (lc *lowerCtx) playerTurnCondition(ps *params, v string) (lCond, error) {
	if beside := conditionKeysBesides(ps, "ConditionPlayerTurn", "ConditionPhases"); len(beside) > 0 && !isDefinedPresentPair(beside) {
		return lCond{}, refuse("ConditionPlayerTurn$ beside %s$", beside[0])
	}
	turn, err := lc.turnPhasesCondition(ps, v)
	if err != nil {
		return lCond{}, err
	}
	if _, ok := ps.m["ConditionPresent"]; !ok {
		return turn, nil
	}
	group, err := lc.definedPresentGroup(ps)
	if err != nil {
		return lCond{}, err
	}
	return andCond(turn, group), nil
}

// isDefinedPresentPair reports whether the Condition*$ keys beside the
// turn/phase gate are exactly ConditionDefined$ and ConditionPresent$.
func isDefinedPresentPair(keys []string) bool {
	return len(keys) == 2 && keys[0] == "ConditionDefined" && keys[1] == "ConditionPresent"
}

// definedPresentGroup lowers the ConditionDefined$+ConditionPresent$ group
// beside a turn/phase gate through nodeCondition's own present-group arm, on
// a params holding only the pair. gorge conditions.go AND-s the turn/phase
// preconditions (extraMet) with that group gate's answer (combine), so the
// group reads exactly as it does alone: Card.wasCast on ConditionDefined$
// Self is the resolving spell's own cast (kernel wasCastSpell; a copy was
// never cast, CR 707.10).
func (lc *lowerCtx) definedPresentGroup(ps *params) (lCond, error) {
	sub := newParams(map[string]string{"ConditionDefined": ps.m["ConditionDefined"], "ConditionPresent": ps.m["ConditionPresent"]})
	group, err := lc.nodeCondition(sub)
	if err != nil {
		return lCond{}, err
	}
	if un := sub.unread(); len(un) > 0 {
		return lCond{}, refuse("ConditionPlayerTurn$ beside an unread %s$", un[0])
	}
	if group.Kind == cNone {
		return lCond{}, refuse("ConditionPlayerTurn$ beside a ConditionPresent$ group that lowers to no gate")
	}
	ps.consume("ConditionDefined", "ConditionPresent")
	return group, nil
}

// turnPhasesCondition is the turn test, conjoined with ConditionPhases$ when
// the node carries one.
func (lc *lowerCtx) turnPhasesCondition(ps *params, v string) (lCond, error) {
	var turn lCond
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		turn = lCond{Kind: cYourTurn}
	case "false":
		turn = lCond{Kind: cYourTurn, Neg: true}
	default:
		return lCond{}, refuse("ConditionPlayerTurn$ %q", v)
	}
	spec, ok := ps.get("ConditionPhases")
	if !ok {
		return turn, nil
	}
	phases, err := lc.phasesCondition(spec)
	if err != nil {
		return lCond{}, err
	}
	return andCond(turn, phases), nil
}
