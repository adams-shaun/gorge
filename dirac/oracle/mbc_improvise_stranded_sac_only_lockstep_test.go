package oracle

import "testing"

// CR 702.126a, 601.2h, 733.1: Foundry Assembler's {5} is offered at the
// improvise credit of every untapped artifact, and the player announces fewer.
// The kernel then re-planned the payment on what the announced artifacts leave
// and panicked "autopay found no plan" when it found none (tail-45 pair 78 when
// recorded; both recordings replay on tail-35, where the name-hashed pools put
// the card). Gorge answers by what its window census says of the same board:
//   - two Eldrazi Spawn tokens (sacrifice-only {C}) cannot pay the {3} the two
//     announced artifacts leave, so gorge reverses the cast ("cost no longer
//     payable"); the kernel's dry strand check stops at a sacrifice-only source
//     it cannot price, though the plan prices it.
//   - Gilanra's restricted {G} (spent on a creature spell) pays the {3} through
//     gorge's manual window where the plan counts no restricted source, so the
//     window pays the cast, not a reversal.
func TestMBCImproviseStrandedSacOnlyReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"improvise_fewer_picks_than_credit_strands_sac_only_board",
		"improvise_plan_fails_where_restricted_mana_pays",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
