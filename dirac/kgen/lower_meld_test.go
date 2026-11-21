package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func meldPoolOf(t *testing.T, commander bool, names ...string) *lowered {
	t.Helper()
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	p.commander = commander
	return lowerPool(p)
}

func findLCard(lw *lowered, name string) *lCard {
	for _, c := range append(append([]*lCard(nil), lw.cards...), lw.tokens...) {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestKgenMeldResultCost (CR 712.8g, 202.3c): the melded face's mana cost is
// the sum of the two front faces' mana values (Graf Rats {1}{B} 2 +
// Midnight Scavengers {4}{B} 5), generic only, and Graf Rats' own Meld lowers
// to OpExt ExtMeld over the partner's named filter.
func TestKgenMeldResultCost(t *testing.T) {
	lw := meldPoolOf(t, false, "Graf Rats", "Midnight Scavengers")
	front := findLCard(lw, "Graf Rats")
	if front == nil || len(front.Refusals) != 0 || front.FaceMode != kop.FaceMeld {
		t.Fatalf("Graf Rats: %+v", front)
	}
	res := findLCard(lw, faceKey("Graf Rats", 1))
	if res == nil || len(res.Refusals) != 0 {
		t.Fatalf("Chittering Host face entry: %+v", res)
	}
	if got, want := res.Cost.Generic, 2+5; got != want || res.Cost.Pips != [5]int{} {
		t.Errorf("Chittering Host cost = %+v, want generic %d and no pips", res.Cost, want)
	}
	found := false
	for _, ab := range front.Abilities {
		for _, in := range ab.Prog {
			found = found || in.Op == "Meld" && in.F != nil
		}
	}
	if !found {
		t.Errorf("Graf Rats lowers no Meld instruction")
	}
}

// TestKgenMeldCommanderPool: a Meld result card is refused by name in a
// Commander pool until CR 903.3 is mirrored (part 2), and Tapped$/Attacking$
// refuses everywhere.
func TestKgenMeldCommanderPool(t *testing.T) {
	lw := meldPoolOf(t, true, "Graf Rats", "Midnight Scavengers")
	c := findLCard(lw, "Graf Rats")
	if c == nil || len(c.Refusals) == 0 || c.Refusals[0].Reason != meldCommanderRefusal {
		t.Fatalf("Graf Rats in a Commander pool: %+v", c)
	}
	if m := findLCard(lw, "Midnight Scavengers"); m == nil || len(m.Refusals) != 0 {
		t.Errorf("the pair's other half refused: %+v", m)
	}
	mish := findLCard(meldPoolOf(t, false, "Mishra, Claimed by Gix", "Phyrexian Dragon Engine"), "Mishra, Claimed by Gix")
	refused := false
	for _, r := range mish.Refusals {
		refused = refused || strings.Contains(r.Reason, "Tapped$/Attacking$")
	}
	if !refused {
		t.Errorf("Mishra's attacking meld not refused: %+v", mish.Refusals)
	}
}
