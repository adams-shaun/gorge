package oracle

import (
	"slices"
	"testing"
)

// TestRefusedParentLeavesTheShardsTheirEntries: TestRefusedEntriesStillDiverge, the
// dirac gate's entry (scripts/dirac-gate.sh names it alone), checks the entries of
// every shard its process's -run does not also select, and none when the shards run
// beside it, so the gate checks every entry and no process checks one twice.
func TestRefusedParentLeavesTheShardsTheirEntries(t *testing.T) {
	all := []int{0, 1, 2, 3, 4, 5, 6, 7}
	for _, c := range []struct {
		run  string
		left []int
	}{
		{`^TestRefusedEntriesStillDiverge$|^TestChoosesHaveNoOther$|^TestStd[A-Za-z]*ReprosLockstep`, all}, // the gate's step 8
		{`^TestRefusedEntriesStillDiverge$/^Some Card$`, all},                                              // an isolate child
		{``, nil},                           // a whole-package run
		{`.`, nil},                          // ditto
		{`RefusedEntriesStillDiverge`, nil}, // test-smart's narrow census, unanchored
		{`^TestRefusedEntriesStillDiverge[0-9]+$`, nil}, // the lanes, CI and regen-main
		{`^(TestRefusedEntriesStillDiverge|TestRefusedEntriesStillDiverge3)$`, []int{0, 1, 2, 4, 5, 6, 7}},
		{`^TestRefusedEntriesStillDiverge([0-3])?$/x`, []int{4, 5, 6, 7}},  // a '/' only past the group
		{`^TestRefusedEntriesStillDiverge[0/1]$`, []int{2, 3, 4, 5, 6, 7}}, // a '/' in a class is not a split
		// go splits on a top-level '|' before '/': each alternative's first element
		// selects top-level tests, so the shard named after the '/' runs beside the parent.
		{`^TestA$/x|^TestRefusedEntriesStillDiverge$|^TestRefusedEntriesStillDiverge3$`, []int{0, 1, 2, 4, 5, 6, 7}},
		{`(^TestA$|^TestB$)/x|^TestRefusedEntriesStillDiverge[0-6]$`, []int{7}}, // a '|' in a group is not a split
	} {
		left := refusedShardsLeft(c.run)
		var got []int
		for i := range refusedShards {
			if left[i] {
				got = append(got, i)
			}
		}
		if !slices.Equal(got, c.left) {
			t.Errorf("-run %q: parent checks shards %v, want %v", c.run, got, c.left)
		}
	}
}
