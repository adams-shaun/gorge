package oracle

import "testing"

// CR 700.2a with CR 700.2d: Cabaretti Confluence ("choose three, you may
// choose the same mode more than once") lists only the modes that can be
// chosen; a mode whose mandatory target has no candidate is left out and the
// rest keep a dense index. After two picks of the first mode the third
// modes decision parted: gorge offered [mode 0, mode 1] and the kernel
// [mode 0, mode 2] (decision 838 of pair 4, surface autopay). The recording
// is on a custom pool, the table its own decks need: the tail buckets a
// synthetic pool deals re-cut whenever the lowered card set moves, and this
// pair's Ancient Den already moved out of its recorded bucket once.
func TestMBCCabarettiConfluenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cabaretti_confluence_repeat_mode_dense")
}
