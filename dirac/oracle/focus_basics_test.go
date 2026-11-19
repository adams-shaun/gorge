package oracle

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// firstFocusSpell is the first lowered nonbasic spell of set, the card a focus line can name.
func firstFocusSpell(t *testing.T, set cardset.Set) string {
	t.Helper()
	reg := synthReg(t)
	for i, n := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusNotLowered || cardset.IsSynthBasic(n) {
			continue
		}
		if c, ok := reg.Lookup(n); ok && kindOf(c) == synthSpell {
			return n
		}
	}
	t.Fatal("the set lowers no spell")
	return ""
}

// TestFocusPoolSkipsBasicLines: an evidence list the loop writes names the five basics
// beside the cards it credits, and a basic is never a playable focus card, so the deal
// used to refuse every such list ("Forest" is not a card tail-00 plays). The basics are
// skipped; the deal is the one the list without them makes, and a list of nothing else is
// an error.
func TestFocusPoolSkipsBasicLines(t *testing.T) {
	set := cardsettest.Synth(t, "tail-00")
	reg := synthReg(t)
	spell := firstFocusSpell(t, set)
	plain := []FocusCard{{Name: spell}}
	withBasics := []FocusCard{{Name: "Forest"}, {Name: spell}, {Name: "Island"}, {Name: "Swamp"}}
	want, wantFiles, err := FocusPool(reg, set, "f", plain, 4, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	got, gotFiles, err := FocusPool(reg, set, "f", withBasics, 4, 1, 4)
	if err != nil {
		t.Fatalf("a list naming basics beside %q: %v", spell, err)
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatal("basic lines changed the deal")
	}
	if len(withBasics) != 4 || withBasics[0].Name != "Forest" {
		t.Fatal("the caller's list was changed")
	}
	_, _, err = FocusPool(reg, set, "f", []FocusCard{{Name: "Forest"}, {Name: "Plains"}}, 4, 1, 4)
	if err == nil || !strings.Contains(err.Error(), "no card but basic lands") {
		t.Fatalf("a list of basics only: err = %v", err)
	}
}
