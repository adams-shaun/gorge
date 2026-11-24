package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MTGA combat-props family: combat referents and
// riders read relative to the resolving combat trigger.

// triggeredDefenderOrPW is Raid Bombardment's damage recipient, "the player
// or planeswalker that creature is attacking". gorge (effects/context.go
// definedSpec) splits it on " & ": TriggeredDefender.Opponent is the
// Attacks event's DefendingPlayer -- the defending SEAT -- and, attacking a
// planeswalker or battle, Valid Planeswalker.TriggeredDefender is that object
// (DefenderObj, bound per instance as the trigger is queued) while the
// .Opponent arm names nobody; the damage hits the attacked object then. The
// lowering is a DealDamage on the defending player (two-player: the
// controller's opponent; past two seats the seat of the Attacks event that
// queued the trigger, stackEntry.defSeat: seat-exact) marked DmgAtkedObj, behind
// OpAtkPWGuard C AtkPWHit, which stalls the game when the attacker left
// while attacking an object or attacks the trigger's controller, where
// gorge's .Opponent qualifier names nobody; a gone attacker that attacked a
// player passes, on the queue-time defSeat. Mage Slayer's DamageSource$
// DealDamageFrom (CS defending) damages that object too, behind the same
// guard: the kernel's defending mode deals through dmgSrcObject
// (act_damagefrom.go).
const triggeredDefenderOrPW = "TriggeredDefender.Opponent & Valid Planeswalker.TriggeredDefender"

func (lc *lowerCtx) triggeredDefenderOrPW(v string) (lRef, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Attacks" {
		return lRef{}, refuse("Defined$ %q outside an Attacks trigger", v)
	}
	lc.emit(lInstr{Op: "AtkPWGuard", C: kop.AtkPWHit, Note: "the attacker attacks a player or planeswalker/battle (else the game stalls)"})
	return lRef{Kind: rDefending, N: 1}, nil // N 1: damage reaches the attacked planeswalker/battle (opDealDamage)
}

// tokenAttackingRemembered is Adeline's TokenAttacking$ inside a RepeatEach
// over the opponents: gorge tokenAttackingRider attacks the repeated
// opponent, or asks (CR 508.4) which of that player's planeswalkers it
// attacks when there are any; the kernel attacks the player and stalls
// (StallDefenderPW) when the opponent controls a planeswalker.
// tokAtkPlayerBit is CreateToken's C bit for kop.TokAttackPlayer (with 64).
const tokAtkPlayerBit = 16384

const tokenAttackingRemembered = "RememberedPlayer & Valid Planeswalker.ControlledBy Remembered"

// tokenAttacking lowers TokenAttacking$ (gorge effects/token.go
// tokenAttackingRider) to CreateToken's C bits: 64 the token enters
// attacking the controller's opponent, 128 the defender is gorge's CR 508.4
// ask (TokenAttacking$ True: resolveEnteringDefender's fallback is the
// attack trigger's DefendingPlayer, asked among the opponent's
// planeswalkers and battles too; the kernel stalls when there are any).
// True is read inside an attack trigger of the controller's own attack
// (charm modes included: Warren Warleader's AttackersDeclared charm) and in a
// spell with no trigger (The Last Ronin's Technique's sneaked branch): the
// kernel tokenAttacks gates the attack on gorge's enteringDefenderFallback
// (the controller's declare attackers .. end of combat), which an attack
// trigger always passes and a spell may not. Any other trigger is refused,
// its defending player being unmodelled. The selector is read only as
// Adeline's RememberedPlayer form over the opponents.
func (lc *lowerCtx) tokenAttacking(v string) (int, error) {
	switch v = strings.TrimSpace(v); {
	case v == "True":
		if lc.ab.Trigger != nil && !lc.ownAttackTrigger() {
			return 0, refuse("TokenAttacking$ True in a trigger other than the controller's own attack")
		}
		if lc.ab.Trigger == nil && lc.ab.Latent {
			// a delayed or reflexive program: gorge may carry the spawning
			// trigger's defending player, which the kernel does not
			return 0, refuse("TokenAttacking$ True in a latent program outside an attack trigger")
		}
		return 64 | 128, nil
	case v == tokenAttackingRemembered:
		if lc.repeatPl == 0 || !lc.repeatOpp {
			return 0, refuse("TokenAttacking$ %q outside a RepeatEach over the opponents", v)
		}
		return 64, nil
	case v == "Remembered" || v == "RememberedPlayer":
		// the plain Remembered family (effects.plainRememberedSelector): the
		// repeated opponent alone, no planeswalker arm and no ask
		if lc.repeatPl == 0 || !lc.repeatOpp {
			return 0, refuse("TokenAttacking$ %q outside a RepeatEach over the opponents", v)
		}
		return 64 | tokAtkPlayerBit, nil
	}
	return 0, refuse("TokenAttacking$ %q", v)
}
