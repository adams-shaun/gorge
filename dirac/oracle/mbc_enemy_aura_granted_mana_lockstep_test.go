package oracle

import "testing"

// CR 303.4 with CR 605.1a: the opponent's Gift of Paradise enchants this
// seat's Plains, so the land's controller can tap it for the granted "{T}: Add
// two mana of any one color" — both casts at the divergence (Battleflight
// Eagle, Scour the Desert) are five mana that only that grant pays.
func TestMBCBattleflightEagleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "enemy_aura_granted_mana")
}
