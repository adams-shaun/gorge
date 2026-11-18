package oracle

import "testing"

// CR 115.5 with CR 601.2b-c: a spell on the stack is an illegal target for
// itself, so the X ask (CR 601.2b) offers only the values whose targets the
// target ask (CR 601.2c) could still name. Spell Blast's cmcEQX target at
// X = 1 reads the cast's own mana value, which on the stack is X + the {U}
// pip; with X still unannounced (read 0) that value is 1, and the kernel's
// X ask once counted its own Spell Blast as the CMC-1 target and offered
// X = 1 beside X = 2 (gorge excludes the cast's card from its own targets,
// xAsk's targetsAnnounceableAt; decision 329, pair 36 of tail-66's ev-tail-66
// focus run).
//
// CR 107.3 with CR 601.2b: the offer gate must also agree on whether any
// payable X has a legal target. With three mana and a lone three-mana
// Ferropede on the stack, Spell Blast needs X = 3 ({3}{U}) and is not
// castable; dirac offered it anyway (decision 634, pair 35 of tail-13's
// tail-13-c000 run).
func TestMBCSpellBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xask_self_spell_target")
	stdReproNamed(t, "spell_blast_x_target_unpayable")
}
