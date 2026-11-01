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
// True is read only inside an attack trigger of the controller's own
// attack (the fallback is then the controller's opponent); the selector
// only as Adeline's RememberedPlayer form over the opponents.
func (lc *lowerCtx) tokenAttacking(v string) (int, error) {
	if lc.mode >= 0 {
		// a charm mode's mint (Warren Warleader) is not lockstep-proven:
		// TestMtokatkLockstep diverged on that card's offspring cast offer
		// before reaching it (report)
		return 0, refuse("TokenAttacking$ in a charm mode")
	}
	switch v = strings.TrimSpace(v); {
	case v == "True":
		if !lc.ownAttackTrigger() {
			return 0, refuse("TokenAttacking$ True outside an attack trigger of the controller's own attack")
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
