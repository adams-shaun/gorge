package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Bubbling Muck ("until end of turn, whenever a player taps a Swamp for mana,
// that player adds an additional {B}") is refused by name, not lowered to an
// end-of-turn StatTapExtra emblem: gorge registers the trigger as a delayed
// one its payment planner defers the Swamps for (engine/rules
// TestPaymentPlanInterferenceDelayedTapTriggerDefersItsSources) and
// whose extra its potential bound omits, so a kernel that prices the extra was
// offered casts gorge did not (the lowering was tried against this fixture's
// decks: 9 of the first 50 seeds diverged at the options).
func TestMBCBubblingMuckLowers(t *testing.T) {
	corpusDirOrSkip(t)
	set, err := cardset.RawByName("mbcmuck")
	if err != nil {
		t.Fatal(err)
	}
	i, ok := set.Lookup("Bubbling Muck")
	if !ok {
		t.Fatal("Bubbling Muck is not in the mbcmuck fixture")
	}
	if got := set.Status(i); got != cardset.StatusNotLowered {
		t.Fatalf("Bubbling Muck status = %v, want not lowered", got)
	}
	const want = "Effect TapsForMana trigger until end of turn: gorge defers the tapped source"
	if r := set.Reason(i); !strings.Contains(r, want) {
		t.Fatalf("Bubbling Muck refusal = %q, want it to name %q", r, want)
	}
}
