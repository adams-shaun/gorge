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
//
// Boast$ True (CR 702.142a) is lowered here too: gorge's boastGateOK reads
// the source's own AttacksThisTurn and its activations this turn, so the gate
// is the kernel's mechAttackedObj mask with a Card.Self filter (the same
// per-object read; a blinked source is a new object, CR 400.7) beside
// TurnOnce, the once-each-turn limit.
func (lc *lowerCtx) activationWindow(ps *params, sa *cards.SA, ab *lAbility) error {
	if ab.Ninjutsu || ab.PutSelf {
		return nil // the ninjutsu expansion's own window (lower_ninjutsu.go)
	}
	if v, ok := ps.get("Boast"); ok {
		if v != "True" || ab.Kind != akActivated || ab.Zone != 0 {
			return refuse("Boast$ %q outside a printed battlefield activated ability", v)
		}
		f, err := lc.filter("Card.Self")
		if err != nil {
			return err
		}
		ab.ActCond = andCond(ab.ActCond, lCond{Kind: cValue,
			A: lValue{Kind: vAttackedTurn, F: f}, Op: opGE, B: cst(1)})
		ab.TurnOnce = true
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

// spellWindow lowers a spell's own timing restriction, the keys gorge's
// activationPhasesOK reads on the spell's SA at the cast offer
// (spellTimingPre, CR 601.3): ActivationPhases$ ("Cast this spell only during
// combat", Savage Beating, Panic), ActivationFirstCombat$ True, PlayerTurn$
// True and OpponentTurn$ True ("only during your turn" / "an opponent's
// turn", Take the Bait). Each rides the spell ability's cast restriction
// (kernel castForbidden, cast_spellcondition.go) beside IsPresent$'s.
// ActivationAfterBlockers$ stays unread, so it refuses with its name.
func (lc *lowerCtx) spellWindow(ps *params, ab *lAbility) error {
	if spec, ok := ps.get("ActivationPhases"); ok {
		c, err := phasesCond("ActivationPhases", spec)
		if err != nil {
			return err
		}
		ab.ActCond = andCond(ab.ActCond, c)
	}
	if v, ok := ps.get("ActivationFirstCombat"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("ActivationFirstCombat$ %q", v)
		}
		ab.ActCond = andCond(ab.ActCond, lCond{Kind: cValue, A: lValue{Kind: vPhaseCount, N: kop.PhaseCountCombats}, Op: opLE, B: cst(1)})
	}
	if v, ok := ps.get("PlayerTurn"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("spell PlayerTurn$ %q", v)
		}
		ab.ActCond = andCond(ab.ActCond, lCond{Kind: cYourTurn})
	}
	if v, ok := ps.get("OpponentTurn"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("spell OpponentTurn$ %q", v)
		}
		ab.ActCond = andCond(ab.ActCond, lCond{Kind: cYourTurn, Neg: true}) // not the caster's turn: an opponent's (no teammates)
	}
	return nil
}
