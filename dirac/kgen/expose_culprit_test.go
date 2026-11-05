package kgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestKgenExposeTheCulpritLowers: Expose the Culprit lowers whole -- the
// second mode's any-number hidden pick is bounded by the Count$Valid disguise
// count (withDisguise) and the chained Defined$ Remembered cloak lowers as
// CfCloakRemembered with the pile-shuffle bit.
func TestKgenExposeTheCulpritLowers(t *testing.T) {
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Expose the Culprit\nGrizzly Bears\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	var c *lCard
	for _, lc := range lowerPool(p).cards {
		if lc.Name == "Expose the Culprit" {
			c = lc
		}
	}
	if c == nil {
		t.Fatal("Expose the Culprit not lowered")
	}
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %q", lowerRefusals(c))
	}
	s, err := readSupport(os.DirFS(testRoot))
	if err != nil {
		t.Fatal(err)
	}
	if st, why := statusOf(c, s); st == cardset.StatusNotLowered {
		t.Fatalf("statusOf = NotLowered (%s)", why)
	}
	var pick, cloak bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			switch {
			case in.Op == "ChooseMove" && in.A.Kind == vCount:
				pick = true
			case in.Op == "Cloak" && in.C&kop.CfCloakRemembered != 0 && in.C&kop.CfShuffle != 0:
				cloak = true
			}
		}
	}
	if !pick || !cloak {
		t.Fatalf("second mode: dynamic ChooseMove %v, remembered Cloak with the shuffle bit %v", pick, cloak)
	}
}
