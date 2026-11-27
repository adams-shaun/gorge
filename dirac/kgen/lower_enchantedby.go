package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// phaseEnchantedBy is a Phase trigger's whose (w, ok): ValidPlayer$
// Player.EnchantedBy on a printed battlefield listener is gorge
// playerSpecQualifierEnchantedBy, the player this Aura enchants (Accursed
// Witch's back face); the kernel reads the listener's attachment
// (defEnchanted). Opponent.EnchantedBy adds the Opponent base: the enchanted
// player must also be an opponent of the Aura's controller. Any other shape keeps the whose already read.
func (lc *lowerCtx) phaseEnchantedBy(tp *params, tr *lTrigger, w uint8, ok bool) (uint8, bool) {
	if lc.granted == 0 && (tr.Zone == zNone || tr.Zone == zBattlefield) {
		switch tp.str("ValidPlayer") {
		case "Player.EnchantedBy":
			return whoEnchanted, true
		case "Opponent.EnchantedBy":
			// the Opponent base, then the same qualifier (Overencumbered)
			return whoEnchantedOpp, true
		}
	}
	return w, ok
}

// damageTargetSpec reads a DamageDone trigger's ValidTarget$: the
// attacked-you-last-turn (lower_atklastturn.go), Player.EnchantedBy
// (damageEnchantedBy) and no-contract (lower_contract.go) player specs, else
// the ordinary filter.
func (lc *lowerCtx) damageTargetSpec(tp *params, tr *lTrigger, t cards.Trigger, readF func(string) (*lFilter, error)) (err error) {
	switch tp.str("ValidTarget") {
	case atkLastTurnSpec:
		return atkYouLastTurn(tp, tr, t)
	case "Player.EnchantedBy":
		return lc.damageEnchantedBy(tr)
	case noContractSpec:
		return noContract(tp, tr, t)
	}
	tr.F2, err = readF("ValidTarget")
	return err
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
