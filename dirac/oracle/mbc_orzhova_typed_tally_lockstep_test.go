package oracle

import "testing"

// CR 601.2b, 605.3: Orzhova, the Church of Deals' {2},{T} activation, funded
// through Energy Refractor's fee-only filter beside plain lands, is a legal
// offer. gorge's prefix search keys the floating pool's artifact-typed units by
// count (Player.ManaUnits), so a pool of two {R} with one filter-made unit is
// a different payer state from one with two; dirac keyed them by colour bit on
// a table with no hand-mana card, dedup'd the second, and priced the play in
// fewer nodes than gorge's 160-node budget (decision 1237 of tail-02 pair 13).
func TestMBCOrzhovaTypedArtifactTallyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "typed_artifact_mana_tally_without_hand_mana_dedup")
}
