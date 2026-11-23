package oracle

import "testing"

// CR 305.6 with gorge's intrinsic append (pay.ManaAbilitiesProduce keys on the
// raw Produced$): under Realmwright's chosen Swamp, Cabal Coffers' own
// "{2}, {T}: Add {B} for each Swamp" already produces {B}, so the granted
// type adds no plain {B} beside it and the planner prices Coffers as a census
// gap. Locthwain Lancer ({4}{B}) is then payable only by a scripted prefix,
// which the autopay surface leaves out of the priority list; the kernel had
// offered it (decision 518 of tail-41 pair 7).
func TestMBCLocthwainLancerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fee_dynamic_mana_land_grant_intrinsic_dedup")
}
