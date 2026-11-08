package kgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestToLibraryGraveBottomGated: Fell Horseman's dies trigger puts the
// triggered card from its graveyard on the bottom of its owner's library
// (Origin$ Graveyard, LibraryPosition$ -1). It lowers to OpToLibrary with
// kop.LibBottomGrave, so the kernel leaves a card returned to a hand in
// response where it is: a new object the trigger cannot find (CR 400.7).
// The plain bottom C (1) would move it from the hand.
func TestToLibraryGraveBottomGated(t *testing.T) {
	const card = "Fell Horseman"
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(card+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	i := -1
	for j := range all {
		if all[j].Name == card {
			i = j
		}
	}
	if i < 0 {
		t.Fatalf("%s: not lowered", card)
	}
	if r := lowerRefusals(all[i]); len(r) > 0 {
		t.Fatalf("%s: refused: %v", card, r)
	}
	found := false
	for _, a := range bs[i].def.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpToLibrary || in.Obj != kop.RefTriggerObj {
				continue
			}
			found = true
			if in.C != kop.LibBottomGrave {
				t.Errorf("%s: ToLibrary C = %d, want kop.LibBottomGrave (%d)", card, in.C, kop.LibBottomGrave)
			}
		}
	}
	if !found {
		t.Fatalf("%s: no trigger-object ToLibrary", card)
	}
}
