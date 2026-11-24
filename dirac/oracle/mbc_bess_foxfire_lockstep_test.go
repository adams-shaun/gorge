package oracle

import "testing"

// CR 616.1: Bess, Soul Nourisher attacks, Foxfire's Effect prevents all
// combat damage dealt to and by it this turn, and Divine Presence ("4 or more
// damage is 3 instead") matches the blocker's damage event, so gorge asks the
// affected player which replacement applies first (decision 544, tail-35-c000
// pair 4). The kernel stalled on that ask (replplus).
func TestMBCBessSoulNourisherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevent_effect_damage_set_foxfire_combat")
}
