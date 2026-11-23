package oracle

import "testing"

// CR 106.6: mana a creature makes that may be spent only on creature spells
// and creature abilities (Gwenna, Eyes of Gaea) can't pay an Equipment's
// equip cost. The kernel offered Meteoric Mace's equip {4} on a board whose
// only unrestricted mana was two sources (decision 404 of tail-42-c000
// pair 21).
func TestMBCMeteoricMaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "creature_only_mana_cant_fund_equip")
}
