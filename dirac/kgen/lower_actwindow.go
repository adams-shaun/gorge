package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// activationWindow lowers an activated battlefield ability's ActivationPhases$
// and ActivationFirstCombat$ True (gorge rules activation_phases.go
// activationPhasesOK, a pure offer-time read): the step must be one of the
// named phases (the ONE phase-name parser, state.ParsePhases, mapped onto the
// kernel's steps by phasesCond), and the turn's combat phases begun must be at
// most one, so an extra combat's steps are out of the window. Each rides the
// ability's activation condition beside PlayerTurn$'s. A mana ability and any
// other ActivationFirstCombat$ value refuse (gorge fails the latter closed);
// ActivationAfterBlockers$ stays unread, so it refuses with its name.
func (lc *lowerCtx) activationWindow(ps *params, sa *cards.SA, ab *lAbility) error {
	if ab.Ninjutsu || ab.PutSelf {
		return nil // the ninjutsu expansion's own window (lower_ninjutsu.go)
	}
	_, hasPhases := ps.m["ActivationPhases"]
	_, hasFirst := ps.m["ActivationFirstCombat"]
	if !hasPhases && !hasFirst {
		return nil
	}
	spec, _ := ps.get("ActivationPhases")
	fc, _ := ps.get("ActivationFirstCombat")
	if sa.API == "Mana" || ab.Kind != akActivated {
		return refuse("ActivationPhases$ / ActivationFirstCombat$ outside a non-mana activated ability")
	}
	if hasPhases {
		c, err := phasesCond("ActivationPhases", spec)
		if err != nil {
			return err
		}
		ab.ActCond = andCond(ab.ActCond, c)
	}
	if hasFirst {
		if !strings.EqualFold(strings.TrimSpace(fc), "True") {
			return refuse("ActivationFirstCombat$ %q", fc)
		}
		ab.ActCond = andCond(ab.ActCond, lCond{Kind: cValue, A: lValue{Kind: vPhaseCount, N: kop.PhaseCountCombats}, Op: opLE, B: cst(1)})
	}
	return nil
}
