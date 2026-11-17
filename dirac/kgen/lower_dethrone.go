package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Dethrone (CR 702.105a): "Whenever this creature attacks the player with
// the most life or tied for most life, put a +1/+1 counter on this
// creature." gorge kwDethrone expands the keyword into an Attacks trigger
// (ValidCard$ Card.Self, Dethrone$ True) over DB$ PutCounter, and
// trigmatch.AttacksMatches gates it as the trigger fires: the event's
// defender is a player (an attack on a planeswalker or a battle never
// matches, CR 508.3a) whose life is at least every living player's
// (PlayerHasMostLife). It is a fire-time gate only, never rechecked on
// resolution.
//
// The kernel reads it as Attacked$ Opponent (Tf2AtkOpp: the attacker
// attacks an opponent player, not a planeswalker or battle; the attacking
// player's creature can only attack an opponent) and a fire-time condition
// over the defending player (kop.RefDefendingPlayer: the seat the attacking
// source attacks, the one opponent at two seats): its life is at least the
// controller's and at least the highest living opponent's (ExOppExtreme
// OppExtLife), which is gorge's comparison against every living player.
// The reading is seat-exact, so no two-seat mark.

// dethroneGate reads an Attacks trigger's Dethrone$ v: True only, on
// ValidCard$ Card.Self (the keyword's expansion), beside no Attacked$ (the
// two defender gates would compose). It sets the player gate and reports
// that the fire-time condition (dethroneCond) is owed.
func dethroneGate(v string, tp *params, tr *lTrigger) (bool, error) {
	if v != "True" || tp.str("ValidCard") != "Card.Self" {
		return false, refuse("Attacks Dethrone %q", v)
	}
	if _, ok := tp.get("Attacked"); ok {
		return false, refuse("Attacks Dethrone beside Attacked$")
	}
	tr.Attacked |= atkOpp
	return true, nil
}

// dethroneOrMostLifeGate reads an Attacks trigger's two most-life gates (the
// keyword's expansion, Condition$ AttackedPlayerWithMostLife) and reports
// whether the shared fire-time condition (dethroneCond) is owed.
func dethroneOrMostLifeGate(tp *params, tr *lTrigger) (bool, error) {
	if v, ok := tp.get("Dethrone"); ok {
		return dethroneGate(v, tp, tr)
	}
	return attackedMostLifeGate(tp, tr)
}

// attackedMostLifeGate reads Condition$ AttackedPlayerWithMostLife on an
// Attacks trigger (Scourge of the Throne: "if it's attacking the player with
// the most life or tied for most life"). gorge trigmatch.AttacksMatches reads
// it as the Dethrone gate, PlayerHasMostLife over the declared defender at the
// fire only, so it lowers to the same Attacked$ Opponent gate and dethroneCond.
// Without it the trigger fired whatever the defender's life. The condition is
// owed (true) only on a Self trigger beside no Attacked$.
func attackedMostLifeGate(tp *params, tr *lTrigger) (bool, error) {
	c, ok := tp.get("Condition")
	if !ok || strings.TrimSpace(c) != "AttackedPlayerWithMostLife" {
		return false, nil
	}
	if v := tp.str("ValidCard"); v != "Creature.Self" && v != "Card.Self" {
		return false, refuse("Attacks Condition$ AttackedPlayerWithMostLife on ValidCard$ %q", v)
	}
	if _, has := tp.get("Attacked"); has {
		return false, refuse("Attacks Condition$ AttackedPlayerWithMostLife beside Attacked$")
	}
	tr.Attacked |= atkOpp
	return true, nil
}

// dethroneCond is the fire-time comparison: the defending player's life is
// at least the controller's and at least every living opponent's.
func dethroneCond() lCond {
	def := lValue{Kind: vLifeOf, Ref: lRef{Kind: rDefending}}
	you := lValue{Kind: vLifeOf, Ref: lRef{Kind: rYou}}
	opps := lValue{Kind: vOppExtreme, N: kop.OppExtLife}
	return andCond(lCond{Kind: cValue, A: def, Op: opGE, B: you}, lCond{Kind: cValue, A: def, Op: opGE, B: opps})
}

// faceHasDethroneTrigger reports whether a K:Dethrone face carries its
// expansion (an Attacks trigger with Dethrone$ True), lowered from that line.
func (lc *lowerCtx) faceHasDethroneTrigger() bool {
	for _, t := range lc.face.Triggers {
		if t.Mode == "Attacks" && t.Params["Dethrone"] == "True" {
			return true
		}
	}
	return false
}
