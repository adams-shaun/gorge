package events

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestResolvedAbilityKeyMergesContentEqualBodies pins the identity rule
// Count$ResolvedThisTurn depends on: "this ability" is keyed by the resolving
// root Ability$ body's CONTENT plus its source, never by a *cards.SA pointer.
// cards.Link (and cards.ResolveSVar) PARSES the Execute$ SVar text fresh, so
// one card's two T: lines that share an Execute$ SVar -- Victor, Valgavoth's
// Seneschal's ChangesZone and FullyUnlock lines both Execute$ TrigSurveil --
// carry pointer-distinct but structurally-equal SAs. Pointer identity would
// split their tally (the oracle calls it one ability); content identity merges
// it. The test builds the two SAs the way the linker does and asserts the
// merge, so the next such card is covered structurally rather than by a card
// allowlist.
func TestResolvedAbilityKeyMergesContentEqualBodies(t *testing.T) {
	svars := map[string]string{
		"TrigSurveil": "DB$ Surveil | Amount$ 2 | ConditionCheckSVar$ Resolved | ConditionSVarCompare$ EQ1",
	}
	a := cards.ResolveSVar(svars, "TrigSurveil")
	b := cards.ResolveSVar(svars, "TrigSurveil")
	if a == nil || b == nil {
		t.Fatal("fixture: ResolveSVar produced nil")
	}
	if a == b {
		t.Fatal("precondition: ResolveSVar returned the SAME pointer -- the test cannot distinguish pointer identity from content identity")
	}
	if got, want := ResolvedAbilityKey(7, a), ResolvedAbilityKey(7, b); got != want {
		t.Fatalf("content-equal bodies produced different keys:\n a=%q\n b=%q", got, want)
	}
	// A different source permanent is a different ability with its own tally.
	if ResolvedAbilityKey(7, a) == ResolvedAbilityKey(8, a) {
		t.Fatal("different source ObjIDs produced the same key")
	}
	// A different body is a different ability.
	other := cards.ResolveSVar(map[string]string{"Other": "DB$ Draw | NumCards$ 1"}, "Other")
	if other == nil {
		t.Fatal("fixture: second ResolveSVar produced nil")
	}
	if ResolvedAbilityKey(7, a) == ResolvedAbilityKey(7, other) {
		t.Fatal("different ability bodies produced the same key")
	}
}
