package oracle

import "testing"

// A source whose only mana ability an effect grants (an Ooze token under
// Jaheira's "Tokens you control have {T}: Add {G}") is a census unit of
// gorge's payment census at its battlefield position, classless because
// pay.PlanAlts drops the granted alternative. Every later unit's position, and
// with it each class's materialize limit (members[len-owed]), counts it. The
// kernel's planner skipped such a source in phases 0 and 1, so the positions
// after it were one lower than gorge's; it now registers an inert census unit
// there (mana.go planInertGrant). The recording is a focus-pool game of
// Jaheira and Gelatinous Genesis whose casts are paid beside granted-only
// Oozes: no divergence was found on the unfixed planner either (the shifted
// limits evaluated the same witness), so it pins the census position, not a
// failing game.
func TestMBCGrantOnlySourceCensusPositionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grantonly_source_census_position")
}
