package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// attackLegKey is the dotted referent the Commander curses' second leg names
// for its beneficiary: `TriggeredAttackingPlayer.Opponent+
// controlsCreature.attacking Player.EnchantedBy` (curse_of_opulence's
// DBToken, curse_of_disturbance's DBToken, curse_of_verbosity's DBDraw,
// curse_of_vitality's DBGainLife, curse_of_bounty's DBUntap).
//
// It is NOT an empty set. Forge resolves TriggeredAttackingPlayer to the one
// declaring seat and filters it relative to the curse's controller
// (Player.Opponent is isOpponentOf(sourceController)); the declared attackers
// are the active player's (CR 508.1), so the ordinary "an opponent attacks
// the player I enchanted" case names that opponent, who must receive the
// second token/draw/life/untap ("Each opponent attacking that player does the
// same"). Only the controller attacking the player it enchanted is excluded,
// which is the YourTurn half of the SkipIf guard below (it is that
// controller's turn exactly when the controller is the declaring player).
//
// The selector is read when the leg RESOLVES (gorge's Defined$ is evaluated
// at resolution), so the guard's other half re-reads the board: the trigger
// player must still control a creature attacking the player THIS Aura is
// attached to (kop.W7AttackingEnchTrig). An attacker removed in response to
// the curse's trigger, or the Aura itself leaving, empties the set in gorge
// and skips the leg here.
const attackLegKey = "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"

// lowerAttackLeg models an AttackersDeclared EnchantedBy trigger's second
// leg as the trigger player's effect. It rewrites the beneficiary to the
// bare TriggeredAttackingPlayer referent (the kernel's queue-time trigger
// player at any seat count) and emits a guard that runs the node only when it
// is NOT the controller's own turn (the ability's controller is not among
// "each opponent attacking that player") AND the trigger player still
// controls a creature attacking this Aura's enchanted player (the
// resolution-time half of the selector). The guard's skip length is patched
// after the node lowers (lower.go's chain). The caller's chain loop then lowers the node normally, so every
// param is read or refused by the ordinary API lowering.
func (lc *lowerCtx) lowerAttackLeg(node *cards.SA, p *params) (guard int, err error) {
	param, ok := lc.attackLegParamFor(node, p)
	if !ok {
		return -1, nil
	}
	p.consume(param)
	p.m[param] = "TriggeredAttackingPlayer"
	guard = len(lc.ab.Prog)
	attacker := &lFilter{Clauses: []lClause{{Types: tCreature}}}
	attacker.Clauses[0].setCmp(cmpWord7, 0, int(kop.W7AttackingEnchTrig), "", "")
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cAnd, Sub: []lCond{
		{Kind: cYourTurn, Neg: true},
		{Kind: cPresent, F: attacker, Op: opGE, A: cst(1)},
	}}, Note: "only an opponent still attacking the enchanted player gets the second leg"})
	return guard, nil
}

// attackLegParamFor reports whether node is the curses' second leg on a
// per-defender Player.EnchantedBy trigger, and which param names its
// beneficiary.
func (lc *lowerCtx) attackLegParamFor(node *cards.SA, p *params) (string, bool) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "AttackersDeclared" || tr.DefPlayer != 4 {
		return "", false
	}
	param := ""
	switch node.API {
	case "Token":
		param = "TokenOwner"
	case "Draw", "GainLife", "UntapAll":
		param = "Defined"
	default:
		return "", false
	}
	if strings.TrimSpace(p.str(param)) != attackLegKey {
		return "", false
	}
	return param, true
}
