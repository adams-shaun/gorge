package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// libPlaceLowered lowers a synthetic sorcery rooted at `root` and returns its
// refusal reasons and each emitted instruction as "Op" or "LibPlace/C".
func libPlaceLowered(t *testing.T, root string) (refusals, ops []string) {
	t.Helper()
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op == "LibPlace" {
				ops = append(ops, fmt.Sprintf("LibPlace/%d", in.C))
			} else {
				ops = append(ops, in.Op)
			}
		}
	}
	return
}

// TestLibPlaceBrackets: gorge's CR 401.4 arrangement (effects.libraryPlacementOrder)
// is posed for a ChangeZoneAll into a library and a multi-target ChangeZone to
// its top or bottom, unless the order is random or the library is shuffled
// afterwards; the move is bracketed by ExtLibPlace's arm (the position) and ask.
func TestLibPlaceBrackets(t *testing.T) {
	tgts := "ValidTgts$ Card.YouOwn | TargetMin$ 0 | TargetMax$ 4 | Origin$ Graveyard | Destination$ Library"
	for _, c := range []struct{ name, root, want string }{
		{"sweep", "SP$ ChangeZoneAll | Cost$ B | Origin$ Graveyard | Destination$ Library | Shuffle$ False | ChangeType$ Card.YouCtrl",
			"LibPlace/0 ChangeZoneAll LibPlace/2"},
		{"targets, bottom", "SP$ ChangeZone | Cost$ B | " + tgts + " | LibraryPosition$ -1", "LibPlace/1 ToLibrary LibPlace/2"},
		{"targets, top", "SP$ ChangeZone | Cost$ B | " + tgts + " | LibraryPosition$ 0", "LibPlace/0 ToLibrary LibPlace/2"},
		{"targets, top by default", "SP$ ChangeZone | Cost$ B | " + tgts, "LibPlace/0 ToLibrary LibPlace/2"},
		{"one target", "SP$ ChangeZone | Cost$ B | ValidTgts$ Card.YouOwn | Origin$ Graveyard | Destination$ Library | LibraryPosition$ -1", "ToLibrary"},
		{"TargetMax$ 1", "SP$ ChangeZone | Cost$ B | ValidTgts$ Card.YouOwn | TargetMax$ 1 | Origin$ Graveyard | Destination$ Library", "ToLibrary"},
		{"sweep, shuffled", "SP$ ChangeZoneAll | Cost$ B | Origin$ Graveyard | Destination$ Library | Shuffle$ True | ChangeType$ Card.YouCtrl", "ChangeZoneAll Shuffle"},
		{"sweep to hand", "SP$ ChangeZoneAll | Cost$ B | Origin$ Graveyard | Destination$ Hand | ChangeType$ Card.YouCtrl", "ChangeZoneAll"},
	} {
		t.Run(c.name, func(t *testing.T) {
			refusals, ops := libPlaceLowered(t, c.root)
			if len(refusals) != 0 {
				t.Fatalf("refused: %v", refusals)
			}
			if got := strings.Join(ops, " "); got != c.want {
				t.Fatalf("lowered to %q, want %q", got, c.want)
			}
		})
	}
	if kop.PlaceArmTop != 0 || kop.PlaceArmBottom != 1 || kop.PlaceAsk != 2 {
		t.Fatal("the arm codes are the ToLibrary position codes the bracket forwards")
	}
}

// TestLibPlaceRefusals: a shape the bracket does not cover is refused by name
// rather than lowered with the kernel's fixed order.
func TestLibPlaceRefusals(t *testing.T) {
	tgts := "ValidTgts$ Card.YouOwn | TargetMin$ 0 | TargetMax$ 4 | Origin$ Graveyard | Destination$ Library"
	for _, c := range []struct{ name, root, want string }{
		{"sweep to the bottom", "SP$ ChangeZoneAll | Cost$ B | Origin$ Graveyard | Destination$ Library | LibraryPosition$ -1 | ChangeType$ Card.YouCtrl", "the top only"},
		{"targets whose owner chooses top or bottom", "SP$ ChangeZone | Cost$ B | " + tgts + " | AlternativeDecider$ TargetedOwner | DestinationAlternative$ Library | LibraryPositionAlternative$ -1", "other than the top or bottom"},
		{"remembered set", "SP$ ChangeZone | Cost$ B | Defined$ Remembered | Origin$ Graveyard | Destination$ Library", `Defined$ "Remembered" to a library`},
		{"valid group", "SP$ ChangeZone | Cost$ B | Defined$ ValidHand Card.Red | Origin$ Hand | Destination$ Library", `to a library`},
	} {
		t.Run(c.name, func(t *testing.T) {
			refusals, _ := libPlaceLowered(t, c.root)
			if !strings.Contains(strings.Join(refusals, "\n"), c.want) {
				t.Fatalf("refusals %v, want one containing %q", refusals, c.want)
			}
		})
	}
	t.Run("a remembered set that is shuffled in asks nothing", func(t *testing.T) {
		refusals, _ := libPlaceLowered(t, "SP$ ChangeZone | Cost$ B | Defined$ Remembered | Origin$ Graveyard | Destination$ Library | Shuffle$ True")
		if strings.Contains(strings.Join(refusals, "\n"), "CR 401.4") {
			t.Fatalf("refused for the arrangement: %v", refusals)
		}
	})
}
