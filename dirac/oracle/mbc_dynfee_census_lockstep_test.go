package oracle

import "testing"

// CR 601.2c with 601.2g/h: Fantastic Bounce costs {2} less if it targets a
// tapped creature, and its caster's untapped Cabal Coffers ({2},{T}: add {B}
// for each Swamp) controls no Swamp. An Amount$ that resolves to zero adds no
// mana (CR 605.1), so both engines prune the Coffers from the target census --
// gorge castWindowSourceExcluded's ManaAmountIsZero arm (engine/effects/
// mana_params.go), the kernel's census pricing it the same way -- and cut the
// menu to the affordable candidates while the unreduced {3}{U} is out of the
// plain sources' reach (Mu Yanling, Sky Dancer, Energy Flux beside the tapped
// Pradesh Gypsies).
//
// dynamic_mana_fee_source_census_gap: {U}{W} floating and an untapped Spara's
// Headquarters beside the Coffers. The kernel once admitted only a fee-free
// {T} dynamic source there and offered the tapped creature alone (decision
// 267) while gorge's census stayed open for the Coffers it could not price;
// the recording replays clean now that gorge prunes the zero amount the same
// way. It is a regression pin for the divergence main's ManaAmountIsZero prune
// fixed; it does not exercise any kernel change.
func TestMBCMuYanlingSkyDancerReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"dynamic_mana_fee_source_census_gap"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
