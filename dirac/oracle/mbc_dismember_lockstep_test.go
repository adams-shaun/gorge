package oracle

import "testing"

// CR 601.2h with CR 107.4f: Dismember's {B/P} pips may each be paid with {B}
// or 2 life. With only a Plains, Overflowing Basin ({1},{T}: {G}{U}) and Bog
// Initiate ({1}: {B}) untapped, gorge's planner reaches {B} through both
// filters and offers the black face beside the life face; dirac offered the
// life face alone (tail-49-c000 pair 30, decision 455).
func TestMBCDismemberReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_through_filter_chain")
	// CR 601.2b, 605.3b: at two life under Scald the Islands are Taps-deferred
	// gaps, so the printed cast census holds one unit; gorge's potential walk
	// prices Dismember's {1} from an Island, a {B} pip from Relic of Legends and
	// the other pip as two life, and the kernel left the cast out
	// (tail-49-c000 pair 5, decision 1576).
	stdReproNamed(t, "phyrexian_cast_priced_over_taps_deferred_islands")
}
