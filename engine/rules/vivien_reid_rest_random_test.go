package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// vivienReidForkFix is the oracle-driver commit of the Forge fork that adds
// RestRandomOrder$ True to Vivien Reid's +1. Until FORGE_REF contains it, the
// corpus under .cards still carries the old script.
const vivienReidForkFix = "1957b93c58c5ba9d5362e4fe14bd1100a425cae4"

// TestVivienReidRestGoesRandom pins that Vivien Reid's +1 puts the rest on the
// bottom of the library in a random order (CR 401.4: "in a random order"
// removes the owner's ordering choice), which the Dig resolution reads from
// RestRandomOrder$.
func TestVivienReidRestGoesRandom(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	vivien := lookup(t, reg, "Vivien Reid")

	var plusOne *cards.SA
	for _, f := range vivien.Faces {
		for _, sa := range f.Abilities {
			cost := sa.ParamStr(cards.PKCost)
			if sa.Kind == "AB" && sa.API == "Dig" && strings.Contains(cost, "AddCounter<") && strings.Contains(cost, "LOYALTY") {
				plusOne = sa
			}
		}
	}
	if plusOne == nil {
		t.Fatalf("precondition: Vivien Reid has no Dig ability whose cost adds loyalty")
	}
	got := plusOne.ParamStr(cards.PKRestRandomOrder)
	if got == "" {
		t.Skipf("the pinned corpus under .cards predates the Vivien Reid script fix (fork commit %s)", vivienReidForkFix)
	}
	if got != "True" {
		t.Fatalf("Vivien Reid +1 RestRandomOrder$ = %q, want %q", got, "True")
	}
}
