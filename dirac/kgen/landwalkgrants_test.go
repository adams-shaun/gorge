package kgen

import (
	"sort"
	"strings"
	"testing"
)

// landwalkGrantCards are the cards of the gap4-08 classes: an Aura's landwalk
// grant, a one-shot PumpAll landwalk grant, a landwalk in a level-up band, the
// IgnoreLandWalk static, and granted Skulk (static, Pump, PumpAll, Equipment).
var landwalkGrantCards = []string{
	"Burrowing", "Cave Sense", "Dryad's Favor", "Fishliver Oil", "Leshrac's Rite", "Volcanic Strength",
	"Nature's Cloak", "Halimar Wavewatch",
	"Crevasse", "Deadfall", "Gosta Dirk", "Great Wall", "Lord Magnus", "Quagmire",
	"Staff of the Ages", "Street Savvy", "Undertow", "Ur-Drago",
	"Behind the Scenes", "MacCready, Lamplight Mayor", "Skeleton Key", "The Master, Mesmerist",
}

// TestLandwalkGrantForms: each card lowers with no refusal and builds.
func TestLandwalkGrantForms(t *testing.T) {
	lw := lowerNamedPool(t, landwalkGrantCards)
	if len(lw.cards) != len(landwalkGrantCards) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(landwalkGrantCards))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
}

// TestLandwalkGrantRefusals: the shapes this change leaves refused stay
// refused by name. Magnigoth Treefolk's Landwalk:YourBasic is a spec gorge's
// LandwalkEvades does not read (the Treefolk has no landwalk there). Concerted
// Effort's and Odric, Lunarch Marshal's shared-keyword pumps lower now
// (sharedpumpall_test.go).
func TestLandwalkGrantRefusals(t *testing.T) {
	want := map[string]string{
		"Magnigoth Treefolk": `unknown filter head "YourBasic"`,
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	lw := lowerNamedPool(t, names)
	for _, c := range lw.cards {
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Reason)
		}
		if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), want[c.Name]) {
			t.Errorf("%s: refusals %q, want one naming %q", c.Name, got, want[c.Name])
		}
	}
}
