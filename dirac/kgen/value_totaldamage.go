package kgen

// Count$TotalDamageDoneByThisTurn on a one-hit spell: "you gain life equal to
// the damage dealt this way" (Corrupt, Enter the God-Eternals).
//
// gorge (engine/effects/count_body.go evalTotalDamageDoneThisTurn) sums the
// damage the HOST source has dealt this turn from its DamageDealtThisTurn
// records, so the head reads what actually landed after prevention and
// replacement (CR 120.4), and an instant or sorcery spell that has resolved
// once is a single object that has dealt nothing before. The kernel's
// VLastDamage is the amount the previous damage op landed (resolve.go
// damageTo: after prevention and replacement, 0 when none), so the two agree
// exactly when the spell's own program has dealt damage through one
// OpDealDamage to one target and nothing else. Any other shape (a permanent
// host that deals damage across a turn, several hits, a divided or mapped
// hit, an each-opponent loop whose tally is a sum) refuses.

// totalDamageValue lowers the bare head at the point the reading instruction
// is being lowered: the ability's program so far must be that single hit.
func (lc *lowerCtx) totalDamageValue() (lValue, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell || lc.trigBody || lc.out.Types&tPermanent != 0 {
		return lValue{}, refuse("Count$TotalDamageDoneByThisTurn outside an instant or sorcery's own spell ability")
	}
	p := lc.ab.Prog
	if len(p) != 1 || p[0].Op != "DealDamage" || p[0].Note != "" || p[0].CS != "" || p[0].Obj.Kind != rTarget {
		return lValue{}, refuse("Count$TotalDamageDoneByThisTurn other than right after the spell's one DealDamage to a target")
	}
	t := lc.ab.Targets[p[0].Obj.N]
	if t.Divided != 0 || t.Max.Kind != vNone && !(t.Max.isConst() && t.Max.N <= 1) {
		return lValue{}, refuse("Count$TotalDamageDoneByThisTurn over a multi-target hit")
	}
	return lValue{Kind: vLastDamage}, nil
}
