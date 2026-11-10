package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestSacrificedTypeSpecTwoSubtypes: a sacrificed-count spec naming two
// creature types is not the one-type form the kernel's snapshot count reads,
// so it keeps its refusal now that a clause may carry a second subtype.
func TestSacrificedTypeSpecTwoSubtypes(t *testing.T) {
	lc := &lowerCtx{p: &pool{}, known: func(string) bool { return true }}
	if _, ok := lc.sacrificedTypeFilter("Creature.Klingon"); !ok {
		t.Fatal("precondition: the one-type spec must lower")
	}
	if _, ok := lc.sacrificedTypeFilter("Creature.Klingon+Human"); ok {
		t.Error("Creature.Klingon+Human lowered, want refused")
	}
}

// TestEffectSourceTriggerRefusalIsStable: with several bad EffectSource keys
// the refusal names the same one every run (the text reaches the census).
func TestEffectSourceTriggerRefusalIsStable(t *testing.T) {
	tr := cards.Trigger{Mode: "DamageDone", Params: map[string]string{
		"ValidTarget": "Card.EffectSource", "ValidCard": "Card.EffectSource",
		"IsPresent": "Card.EffectSource", "ValidSource": "Card.EffectSource",
	}}
	for i := 0; i < 50; i++ {
		_, _, err := effectSourceTrigger(newParams(map[string]string{}), tr)
		if err == nil || !strings.Contains(err.Error(), "IsPresent") {
			t.Fatalf("run %d: got %v, want the first key in sorted order (IsPresent)", i, err)
		}
	}
}
