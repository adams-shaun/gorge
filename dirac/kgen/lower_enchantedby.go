package kgen

// phaseEnchantedBy reports a Phase trigger's ValidPlayer$ Player.EnchantedBy
// on a printed battlefield listener: gorge playerSpecQualifierEnchantedBy,
// the player this Aura enchants (Accursed Witch's back face); the kernel
// reads the listener's attachment (defEnchanted).
func (lc *lowerCtx) phaseEnchantedBy(tp *params, tr *lTrigger) bool {
	return tp.str("ValidPlayer") == "Player.EnchantedBy" && lc.granted == 0 && (tr.Zone == zNone || tr.Zone == zBattlefield)
}

// damageEnchantedBy lowers a DamageDone ValidTarget$ Player.EnchantedBy:
// gorge playerSpecQualifierEnchantedBy, the damaged player is the one this
// Aura enchants (Curse of Stalked Prey); the kernel's damage fire reads the
// listener's attachment (defEnchanted).
func (lc *lowerCtx) damageEnchantedBy(tr *lTrigger) error {
	if lc.granted != 0 || tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("ValidTarget$ Player.EnchantedBy off a printed battlefield trigger")
	}
	tr.F2 = &lFilter{Players: plBoth}
	tr.Whose = whoEnchanted
	return nil
}
