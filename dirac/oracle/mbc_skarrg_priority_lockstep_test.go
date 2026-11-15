package oracle

import "testing"

// CR 605.3a with CR 601.2g: Skarrg, the Rage Pits' {R}{G}, {T} ability is
// payable beside an Eldrazi Spawn token and an Artifact Unknown Shores
// ({1}, {T}: any colour): gorge's planner counts the Spawn's sacrifice-only
// mana ability as a last-resort census unit (pay.PaymentPlanShapeTierOf), not
// a gap, so its prefix search pays the {G} through the Shores. The kernel
// flagged the Spawn a census gap (censusGapMana), which kept the filter beside
// it off the script path, and left the activation out. The recording is the
// first divergence of dr-mbc tail-10 pair 934 (decision 932, options options,
// gorge leading, autopay surface, mulligans 1, seed 1).
func TestMBCSkarrgTheRagePitsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_source_is_no_census_gap_beside_filter")
}
