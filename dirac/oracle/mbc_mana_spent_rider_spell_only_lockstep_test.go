package oracle

import "testing"

// CR 601.2h and CR 602.2: "when you spend this mana to cast a spell" is a
// spell-cast rider, and paying an activated ability's cost is not casting a
// spell. Gilanra, Caller of Wirewood's mana paid Bookwurm's graveyard
// activation ({2}{G}), and gorge queued the SpellCast-mode draw rider because
// the activation's source (a mana value 8 card) satisfied Card.cmcGE6; the
// kernel, correctly, did not (dr-mbc-baab pair 134, decision 822: NStack 2 vs
// 1).
func TestMBCManaSpentRiderSpellOnlyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_mana_spent_rider_spell_only")
}
