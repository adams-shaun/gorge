package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// handBottomSynth lowers a synthetic sorcery whose root is root (a ChangeZone),
// the shape a markerless hand put-back lowers to.
func handBottomSynth(t *testing.T, root string) *lCard {
	t.Helper()
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"},
		Oracle: "You may put a card from your hand on the bottom of your library.", Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestHandLibraryBottom: Manhole Missile's "you may put a card from your hand
// on the bottom of your library" lowers as an OpChooseMove from the hand into
// the library carrying SrHandBottom and RememberChanged$'s SrRemember. A
// LibraryPosition$ 2 hand move is still refused (out of scope), and a Bottom
// flag on any other origin is refused at build.
func TestHandLibraryBottom(t *testing.T) {
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Manhole Missile"}))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for i, c := range all {
		if c.Name != "Manhole Missile" {
			continue
		}
		if rs := lowerRefusals(c); len(rs) != 0 {
			t.Fatalf("Manhole Missile refused: %v", rs)
		}
		if len(bs[i].refusals) != 0 {
			t.Fatalf("Manhole Missile refused at build: %v", bs[i].refusals)
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op != kop.OpChooseMove || kop.Zone(in.C&7) != kop.ZoneLibrary {
					continue
				}
				found++
				if in.B.N&kop.SrHandBottom == 0 {
					t.Errorf("hand-to-library pick lacks SrHandBottom: B %#x", in.B.N)
				}
				if in.C>>3&kop.SrRemember == 0 {
					t.Errorf("RememberChanged$ did not set SrRemember: C %#x", in.C)
				}
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d hand-to-library picks, want the card's one", found)
	}

	t.Run("position 2 still refuses", func(t *testing.T) {
		got, _ := czLowered(t, "SP$ ChangeZone | Origin$ Hand | Destination$ Library | LibraryPosition$ 2 | ChangeNum$ 1")
		if len(got) == 0 {
			t.Fatal("LibraryPosition$ 2 lowered instead of refusing")
		}
	})

	t.Run("Bottom from a graveyard refuses at build", func(t *testing.T) {
		lc := handBottomSynth(t, "SP$ ChangeZone | Origin$ Hand | Destination$ Library | LibraryPosition$ -1 | ChangeNum$ 1")
		if len(lc.Refusals) != 0 {
			t.Fatalf("lowering refused: %v", lc.Refusals)
		}
		changed := false
		for i := range lc.Abilities {
			for j := range lc.Abilities[i].Prog {
				in := &lc.Abilities[i].Prog[j]
				if in.Op != "ChooseMove" || !strings.Contains(in.CS, "from=Hand") {
					continue
				}
				in.CS = strings.Replace(in.CS, "from=Hand", "from=Graveyard", 1)
				changed = true
			}
		}
		if !changed {
			t.Fatal("the synthetic card lowered no hand ChooseMove")
		}
		b := buildCard(newPoolTables(&lowered{cards: []*lCard{lc}}, nil), lc)
		if len(b.refusals) == 0 {
			t.Fatal("Bottom with a graveyard origin built")
		}
		if !strings.Contains(b.refusals[0].Reason, "Bottom") {
			t.Errorf("build refusal %q does not name Bottom", b.refusals[0].Reason)
		}
	})
}
