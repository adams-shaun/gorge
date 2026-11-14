package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/effects"
)

// TestLandTypeWordsMatchGorge: the kernel's land type list (the subtypes a CR
// 305.7 setter drops, dirac.LandTypeWords) is gorge's.
func TestLandTypeWordsMatchGorge(t *testing.T) {
	want := slices.Clone(effects.TypeChoiceLabels("Land", "", ""))
	slices.Sort(want)
	if got := dirac.LandTypeWords(); !slices.Equal(got, want) {
		t.Fatalf("dirac land types %v, gorge's %v", got, want)
	}
}
