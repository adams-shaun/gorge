package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// DealDamage `DamageSource$ <trigger object>` to each
// opponent ("that creature deals damage to each opponent": Champion of the
// Path, Pyrotechnic Performer, Ingris Stingerquill). The damage source is
// the entry's trigger object, live or its last known information; the
// recipients are the resolving controller's opponents (gorge newDamageRider,
// effects/damage.go). Lowered to DealDamageFrom's player mode (CS "players":
// build_ab.go sets B.N 1, dirac/act_damagefrom.go).
//
// Exactly: source rTriggered, recipient rEachOpp, or rDefending (Mage
// Slayer's "it deals damage equal to its power to the player or planeswalker
// it's attacking": the Attacks trigger's defending seat behind the C AtkPWHit
// guard the Defined$ emitted, lower_tokenattacking.go; CS "defending", the
// kernel's player mode over RefDefendingPlayer, which damages the attacked
// object instead of the player when the attack was on one -- act_damagefrom.go).
// A rDefending whose program carries no AtkPWHit guard keeps the plain one.
// Every other recipient (You, a player target slot, TargetController) keeps
// refusing by name.

// progHasAtkPWHit reports whether a program already emits the OpAtkPWGuard
// C AtkPWHit a triggeredDefenderOrPW Defined$ puts ahead of the damage
// (lower_tokenattacking.go): the kernel's defending mode then damages the
// attacked object behind it (act_damagefrom.go), and no second guard goes
// into the program.
func progHasAtkPWHit(prog []lInstr) bool {
	for _, in := range prog {
		if in.Op == "AtkPWGuard" && in.C&kop.AtkPWHit != 0 {
			return true
		}
	}
	return false
}

// dmgSrcPlayers emits the player-mode DealDamageFrom, or refuses.
func (lc *lowerCtx) dmgSrcPlayers(ps *params, dmgSrc, obj lRef, n lValue) error {
	if ps.is("DamageMap", "True") {
		return refuse("DamageMap$ True with DamageSource$: the kernel has no damage-map flush")
	}
	if _, ok := ps.get("DividedAsYouChoose"); ok {
		return refuse("DamageSource$ to each opponent with DividedAsYouChoose$")
	}
	if dmgSrc.Kind != rTriggered || obj.Kind != rEachOpp && obj.Kind != rDefending {
		return refuse("DamageSource$ %s to %s", dmgSrc, obj)
	}
	if obj.Kind == rDefending {
		if obj.N == 1 && !progHasAtkPWHit(lc.ab.Prog) {
			// DealDamageFrom has no planeswalker recipient of its own: a
			// rDefending the Defined$ did not guard (TriggeredDefendingPlayer,
			// TriggeredAttackedTarget) keeps the plain guard, which stalls on
			// an object attack. The triggeredDefenderOrPW shape (Mage Slayer)
			// already carries C AtkPWHit, and the kernel's defending mode
			// damages the attacked object behind it (act_damagefrom.go).
			lc.emit(lInstr{Op: "AtkPWGuard", Note: "the attacker attacks a player (else the game stalls)"})
		}
		lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: 0, CS: "defending", Note: "trigger object deals A to the defending player"})
		return nil
	}
	lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: 0, CS: "players", Note: "trigger object deals A to each opponent"})
	return nil
}

// dmgSrcAnyTarget emits the any-target form of the trigger-object
// DealDamageFrom (Scourge of Valkas: "it deals X damage to any target"): the
// trigger object is the damage source and the one target slot names a player
// or a permanent (CS "anytgt": the kernel's player mode with a slot in C,
// dirac/act_damagefrom.go). Exactly source rTriggered; a divided or
// damage-map form refuses.
func (lc *lowerCtx) dmgSrcAnyTarget(ps *params, dmgSrc, obj lRef, n lValue) error {
	if ps.is("DamageMap", "True") {
		return refuse("DamageMap$ True with DamageSource$: the kernel has no damage-map flush")
	}
	if _, ok := ps.get("DividedAsYouChoose"); ok {
		return refuse("DamageSource$ to a player-capable slot with DividedAsYouChoose$")
	}
	if dmgSrc.Kind != rTriggered || obj.Kind != rTarget {
		return refuse("DamageSource$ %s to %s", dmgSrc, obj)
	}
	lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: obj.N, CS: "anytgt", Note: "trigger object deals A to the target (a player or a permanent)"})
	return nil
}
