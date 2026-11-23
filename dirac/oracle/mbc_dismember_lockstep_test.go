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
}
