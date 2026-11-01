package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRememberCrossAbilityReaders: an ability that reads Remembered without
// writing it in its own program reads another ability's memory, which gorge
// keeps on the source and the kernel does not (per-resolution set), so the
// card refuses. Same-resolution readers stay lowered.
func TestRememberCrossAbilityReaders(t *testing.T) {
	const why = "reads Remembered written by another ability"
	cases := []struct {
		card    string
		refused bool
	}{
		{"Hoarding Dragon", false},    // ETB search-exile remembers; the dies trigger returns it: RemPersist, the dies form
		{"Goblin Plate Mail", false},  // Amass RememberAmass$ + Attach Remembered
		{"Generous Plunderer", false}, // Token RememberOriginalTokens$ + ImmediateTrigger TriggerAmount$ Remembered$Amount
		{"Dance of Many", true},       // ETB copy remembers the token; Leaves triggers exile/watch it
		{"Grindstone", false},         // RepeatWhile over its own remembering mill
		{"Sphinx's Tutelage", false},
		{"Ancestral Blade", false}, // RememberTokens$ read later in its own chain
		{"Cloudshift", false},      // RememberTargets$ flicker in one chain
		{"Cleansing Meditation", false},
		{"Cryptic Coat", false},       // Cloak RememberCloaked$ + Attach Remembered
		{"Conductive Machete", false}, // ManifestDread RememberManifested$ + Attach Remembered
	}
	dir := t.TempDir()
	var names []string
	for _, c := range cases {
		names = append(names, c.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}
	for _, c := range cases {
		lc := byName[c.card]
		if lc == nil {
			t.Errorf("%s: not lowered", c.card)
			continue
		}
		hit := false
		for _, r := range lc.Refusals {
			hit = hit || strings.Contains(r.Reason, why)
		}
		if hit != c.refused {
			t.Errorf("%s: cross-ability Remembered refusal = %v, want %v (%v)", c.card, hit, c.refused, lc.Refusals)
		}
	}
}
