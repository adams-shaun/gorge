package oracle

import "testing"

// CR 605.3a, 602.2b: seventeen untapped Myr Propagators ({3}, {T}: copy itself)
// beside three lands, Ferrous Lake's {1}, {T}: Add {U}{R} among them. The plain
// census cannot pay {3}, so each activation is priced by the filter script, and
// gorge's potential walk searches only the first potentialScriptPlays (16) of
// them, in entry order, leaving the 17th "census_incomplete" and unoffered. The
// kernel priced every activation (actScriptPays) and offered the 17th
// (decision 1198, tail-11-c000 pair 83).
func TestMBCTail11ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_activation_search_slots_cap")
}
