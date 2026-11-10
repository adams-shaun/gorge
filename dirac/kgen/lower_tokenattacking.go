package kgen

import (
	"strings"
)

// The MTGA combat-props family: combat referents and
// riders read relative to the resolving combat trigger.

// triggeredDefenderOrPW is Raid Bombardment's damage recipient, "the player
// or planeswalker that creature is attacking". gorge (effects/context.go
// definedSpec) splits it on " & ": TriggeredDefender.Opponent is the
// Attacks event's DefendingPlayer -- the defending SEAT, also when the
// attacker attacks a planeswalker -- and Valid Planeswalker.TriggeredDefender
// is an unknown predicate there (UnknownPredicates), matching nothing. So
// gorge damages the defending player in every case; that is the card only
// when the attacker attacks a player. The lowering is the defending player
// (two-player: the controller's opponent) behind OpAtkPWGuard, which stalls
// the game when the attacker attacks a planeswalker or battle (or left):
// the kernel never copies the planeswalker case.
const triggeredDefenderOrPW = "TriggeredDefender.Opponent & Valid Planeswalker.TriggeredDefender"

func (lc *lowerCtx) triggeredDefenderOrPW(v string) (lRef, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Attacks" || !lc.ownAttackTrigger() {
		return lRef{}, refuse("Defined$ %q outside an Attacks trigger of the controller's own attack", v)
	}
	lc.emit(lInstr{Op: "AtkPWGuard", Note: "the attacker attacks a player (else the game stalls)"})
	return lRef{Kind: rDefending}, nil
}

// tokenAttackingRemembered is Adeline's TokenAttacking$ inside a RepeatEach
// over the opponents: gorge tokenAttackingPlayers resolves the
// RememberedPlayer arm to the repeated opponent and, the Valid arm being
// an unmodelled object selector, attacks that player with a Note (never
// an ask).
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
	}
	return 0, refuse("TokenAttacking$ %q", v)
}
