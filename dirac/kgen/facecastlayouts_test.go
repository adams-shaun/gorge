package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lowerNamedPool lowers a named list and returns both the front cards and
// their face entries.
func lowerNamedPool(t *testing.T, names []string) *lowered {
	t.Helper()
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	return lowerPool(p)
}

// TestFaceCastLayouts (C5bb): an Adventure card, a split card
// (no Room or Fuse) and an Omen card (CR 720) lower their
// second face as a linked face entry. A Fuse card and a Room stay refused
// as multi-face cards (an Aftermath card's layout is
// aftermath_lower_test.go's). An Adventure with a land front
// and a modal DFC with a nonland back lower (gorge
// offers the Adventure from the hand; resets the back face off the
// stack).
func TestFaceCastLayouts(t *testing.T) {
	accept := map[string]uint8{"Murderous Rider": kop.FaceAdventure, "Brazen Borrower": kop.FaceAdventure,
		"Fire": kop.FaceSplit, "Dead": kop.FaceSplit, "Lindblum, Industrial Regency": kop.FaceAdventure,
		"Pestilent Cauldron": kop.FaceModal, "Kolvori, God of Kinship": kop.FaceModal,
		"Dirgur Island Dragon": kop.FaceOmen,
		"Bloomvine Regent":     kop.FaceOmen} // its face's Shuffle$ False search lowers
	refuse := map[string]string{"Alive": "multi-face card"}
	var names []string
	for n := range accept {
		names = append(names, n)
	}
	for n := range refuse {
		names = append(names, n)
	}
	lw := lowerNamedPool(t, names)
	cards := map[string]*lCard{}
	for _, c := range lw.cards {
		cards[c.Name] = c
	}
	faces := map[string]*lCard{}
	for _, c := range lw.tokens {
		faces[c.Name] = c
	}
	seen := 0
	for _, name := range names {
		c := cards[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		if m, ok := accept[name]; ok {
			seen++
			if len(c.Refusals) > 0 {
				t.Errorf("%s refused: %v", name, c.Refusals)
			}
			if c.FaceMode != m || len(c.Faces) != 2 || c.Faces[1] != faceKey(name, 1) {
				t.Errorf("%s: layout %d faces %v", name, c.FaceMode, c.Faces)
			}
			if f := faces[faceKey(name, 1)]; f == nil || len(f.Refusals) > 0 {
				t.Errorf("%s: face entry %v", name, f)
			}
		}
		if want, ok := refuse[name]; ok {
			seen++
			var got []string
			for _, r := range c.Refusals {
				got = append(got, r.Reason)
			}
			all := strings.Join(got, " | ")
			if f := faces[faceKey(name, 1)]; f != nil {
				for _, r := range f.Refusals {
					all += " | " + r.Reason
				}
			}
			if !strings.Contains(all, want) {
				t.Errorf("%s: refusals %q, want one containing %q", name, all, want)
			}
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}

// TestFaceTokens: a face's tokens ride its front's token list (Curious
// Pair's Treats to Share makes a Food).
func TestFaceTokens(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Curious Pair"}) {
		if len(c.Refusals) > 0 {
			t.Fatalf("refused: %v", c.Refusals)
		}
		food := false
		for _, k := range c.Tokens {
			food = food || strings.Contains(strings.ToLower(k), "food")
		}
		if !food {
			t.Fatalf("Curious Pair's tokens %v carry no Food", c.Tokens)
		}
	}
}
