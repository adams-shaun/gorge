package effects

import "github.com/adams-shaun/gorge/engine/state"

// globalControlReferent resolves the control/ownership referents that read a
// seat straight off the game state and the reading source, not off the
// trigger or resolution context. It returns no target when the referent is
// unbound, which controlReferentPlayers reports as unbound (a predicate fails
// closed, even beneath '!').
//
//   - Player.Active: the active player, whoever's static or ability reads it
//     (Weftwalking's "Card.nonLand+ControlledBy Player.Active").
//   - EnchantedController.isMonarch / EnchantedController.!isMonarch: Forge's
//     Defined player EnchantedController narrowed by a player property
//     (Fall from Favor's "ControlledBy EnchantedController.!isMonarch"): the
//     controller of the permanent the source is attached to, kept only while
//     it is (or is not) the monarch. A source attached to nothing leaves it
//     unbound.
func globalControlReferent(g *state.Game, sc SpecContext, ref string) []state.Target {
	switch controlReferentPlayersCodes.Code(ref) {
	case controlReferentPlayersPlayerActive:
		return []state.Target{{IsPlayer: true, Player: g.Active}}
	case controlReferentPlayersEnchantedControllerMonarch, controlReferentPlayersEnchantedControllerNotMonarch:
		p, ok := playerEnchantedController(g, sc.Source)
		if !ok || g.IsMonarch(p) != (controlReferentPlayersCodes.Code(ref) == controlReferentPlayersEnchantedControllerMonarch) {
			return nil
		}
		return []state.Target{{IsPlayer: true, Player: p}}
	}
	return nil
}

// enchantedControllerPropertyRef reports whether ref is one of the
// EnchantedController player-property spellings globalControlReferent reads;
// controlReferent classifies them as known through it.
func enchantedControllerPropertyRef(ref string) bool {
	switch controlReferentPlayersCodes.Code(ref) {
	case controlReferentPlayersEnchantedControllerMonarch, controlReferentPlayersEnchantedControllerNotMonarch:
		return true
	}
	return false
}
