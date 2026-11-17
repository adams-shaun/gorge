package oracle

import "testing"

// CR 605.3b: a mana ability's rider resolves with its mana. Player 0 casts
// Savage Twister through a prefix that taps Cabal Pit ({T}: add {B}, it deals
// 1 damage to you), turns the {B} into {G} with Shire Scarecrow's filter and
// taps a Mountain. gorge deals the Pit's 1 as it taps; the kernel's filter
// prefix (mana_filter.go filtDo) once added the {B} without the damage, so
// the life totals parted at the spell's X ask (tail-50 Savage Twister focus,
// pair 14, decision 404).
func TestMBCSavageTwisterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_ability_self_damage_before_cast")
}
