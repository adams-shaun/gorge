package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestOffersMemoCheckedRepros replays recordings whose filter search settles a
// Taps trigger a step queued and walks its nodes' offers (Spider-Gwen's loot,
// Manabarbs' deferred taps) with every offer-memo hit walked again (a verdict
// the walk does not give panics): each still agrees with gorge to its end, and
// the memo answered.
func TestOffersMemoCheckedRepros(t *testing.T) {
	reg := synthReg(t)
	defer func(b bool) { dirac.DebugOffersMemoCheck = b }(dirac.DebugOffersMemoCheck)
	dirac.DebugOffersMemoCheck = true
	before := dirac.DebugOffersMemoChecked
	for _, name := range []string{
		"tap_two_trigger_stacks_before_the_play_is_priced",
		"taps_deferred_prefix_one_tap_two_triggers_order",
		"manabarbs_deferred_kicked_cast",
		"cast_tapsdefer_part_paid_pre_push",
		"equip_unpriced_census_under_manabarbs",
		"cast_script_skips_the_lethal_tapsdefer_gaps",
	} {
		r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		if div := replayStdRepro(t, reg, r.Pool, name, r.Surface == "full"); div != nil {
			t.Fatalf("%s: diverged: %v", name, div)
		}
	}
	if dirac.DebugOffersMemoChecked == before {
		t.Fatal("precondition: the offer memo answered no node of these recordings")
	}
	t.Logf("offer memo answered %d nodes, each re-walked", dirac.DebugOffersMemoChecked-before)
}
