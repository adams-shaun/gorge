package kgen

// DealDamage `DamageSource$ <trigger object>` to each
// opponent ("that creature deals damage to each opponent": Champion of the
// Path, Pyrotechnic Performer, Ingris Stingerquill). The damage source is
// the entry's trigger object, live or its last known information; the
// recipients are the resolving controller's opponents (gorge newDamageRider,
// effects/damage.go). Lowered to DealDamageFrom's player mode (CS "players":
// build_ab.go sets B.N 1, dirac/act_damagefrom.go).
//
// Exactly: source rTriggered, recipient rEachOpp. Every other recipient
// (You, a player target slot, TargetController) keeps refusing by name.

// dmgSrcPlayers emits the player-mode DealDamageFrom, or refuses.
func (lc *lowerCtx) dmgSrcPlayers(ps *params, dmgSrc, obj lRef, n lValue) error {
	if ps.is("DamageMap", "True") {
		return refuse("DamageMap$ True with DamageSource$: the kernel has no damage-map flush")
	}
	if _, ok := ps.get("DividedAsYouChoose"); ok {
		return refuse("DamageSource$ to each opponent with DividedAsYouChoose$")
	}
	if dmgSrc.Kind != rTriggered || obj.Kind != rEachOpp {
		return refuse("DamageSource$ %s to %s", dmgSrc, obj)
	}
	lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: 0, CS: "players", Note: "trigger object deals A to each opponent"})
	return nil
}
