package rules

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestConditionTargetedLKICarrierCensus pins the corpus cards that gate a
// power/toughness `ConditionPresent$` on a `ConditionDefined$ Targeted` (or
// `ThisTargetedCard`) member -- the look-back CR 608.2h has to get right.
// Driftgloom Coyote is the only current carrier that moves the target off the
// battlefield before the gate; the rest read a target still in play. A new
// carrier must surface here in review, because the live-vs-LKI distinction is
// invisible when the target has not departed.
func TestConditionTargetedLKICarrierCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	set := map[string]struct{}{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			walkAllSAs(f, func(sa *cards.SA) {
				defined := strings.TrimSpace(sa.Params["ConditionDefined"])
				if defined != "Targeted" && defined != "ThisTargetedCard" {
					return
				}
				// The brief's census spelling: a lowercase power/toughness
				// threshold (`powerLE2`, `toughnessGE6`). The relative
				// `greatestPower`/`leastPower` comparisons are a different
				// look-back class (they compare against the live board), so
				// they stay out of this population by design.
				present := sa.Params["ConditionPresent"]
				if strings.Contains(present, "power") || strings.Contains(present, "toughness") {
					base := filepath.Base(c.Path)
					set[strings.TrimSuffix(base, filepath.Ext(base))] = struct{}{}
				}
			})
		}
	}
	got := make([]string, 0, len(set))
	for name := range set {
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{
		"desperate_plea",
		"dormant_grove_gnarled_grovestrider",
		"driftgloom_coyote",
		"eagle_of_deliverance",
		"ents_fury",
		"hog_monkey_rampage",
		"savage_swipe",
		"strider_ranger_of_the_north",
		"yavimaya_bloomsage_channel",
	}
	if len(got) != len(want) {
		t.Fatalf("ConditionDefined Targeted P/T carrier population = %d, want %d\ngot:  %v\nwant: %v",
			len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("carrier mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}
