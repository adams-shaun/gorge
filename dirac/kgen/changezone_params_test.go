package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// czLowered lowers a synthetic sorcery whose root is `root` and returns its
// refusal reasons and the "Op:CS" of every instruction it emitted.
func czLowered(t *testing.T, root string) (refusals, ops []string) {
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
			ops = append(ops, in.Op+":"+in.CS)
		}
	}
	return
}

// TestChangeZoneParamRefusals: the Mill / ChangeZone choice params are read
// for the shapes gorge honours, and every other value or combination is
// refused with a named reason.
func TestChangeZoneParamRefusals(t *testing.T) {
	for _, c := range []struct{ name, root, want string }{
		{"ParentTarget", "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Exile | ThisDefinedAndTgts$ ParentTarget", `ThisDefinedAndTgts$ "ParentTarget"`},
		{"TopOfLibrary", "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Exile | ThisDefinedAndTgts$ TopOfLibrary", `ThisDefinedAndTgts$ "TopOfLibrary"`},
		{"Self without a target", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Battlefield | ThisDefinedAndTgts$ Self", "without a target"},
		{"known-origin Optional from the graveyard", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Hand | Optional$ True", "ChangeZone Optional$ on a Self Graveyard->Hand move"},
		{"known-origin Optional with Chooser", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Battlefield | Destination$ Hand | Optional$ True | Chooser$ Opponent", "Optional$ with Chooser$ on a known-origin move"},
		{"known-origin Optional exile", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Battlefield | Destination$ Exile | Optional$ True", "ChangeZone Optional$ on a Self Battlefield->Exile move"},
		{"known-origin Optional other value", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Battlefield | Destination$ Hand | Optional$ You", `ChangeZone Optional$ "You"`},
		{"Attacking selector", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Battlefield | Attacking$ Defending", `Attacking$ "Defending"`},
		{"Attacking to hand", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Hand | Attacking$ True", "unread params [Attacking]"},
		{"DifferentNames on a known-origin move", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Hand | DifferentNames$ True", "unread params [DifferentNames]"},
		{"DifferentNames False", "SP$ ChangeZone | Cost$ B | Origin$ Library | Destination$ Hand | ChangeType$ Land.Basic | DifferentNames$ False", `DifferentNames$ "False"`},
		{"Mandatory False", "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Exile | Mandatory$ False", `Mandatory$ "False"`},
		{"Mill Optional You", "SP$ Mill | Cost$ B | Defined$ You | NumCards$ 2 | Optional$ You", `Optional$ "You"`},
		{"graveyard Optional with Mandatory", "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Hand | ChangeType$ Creature.YouOwn | Hidden$ True | Optional$ True | Mandatory$ True", "Optional$ with Mandatory$"},
		{"graveyard Optional other value", "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Hand | ChangeType$ Creature.YouOwn | Hidden$ True | Optional$ Maybe", `Optional$ "Maybe"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
	t.Run("Mandatory with an optional target", func(t *testing.T) {
		got, _ := czLowered(t, "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature.Other | TargetMin$ 0 | TargetMax$ 1 | Mandatory$ True | Origin$ Battlefield | Destination$ Exile")
		kdefWant(t, got, "Mandatory$ with an optional target")
	})
	t.Run("Attacking beside an optional target", func(t *testing.T) {
		got := kdefRefusals(t, "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature.YouOwn | TargetMin$ 0 | TargetMax$ 1 | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True",
			"DB$ ChangeZone | Defined$ Remembered | Origin$ All | Destination$ Battlefield | Tapped$ True | Attacking$ True")
		kdefWant(t, got, "Attacking$ in a chain with an optional target")
	})
	// the lifted shapes lower, carrying the new flags
	for _, c := range []struct{ name, root, want string }{
		{"Mill Optional", "SP$ Mill | Cost$ B | Defined$ You | NumCards$ 2 | Optional$ True", "MayAsk:"},
		{"graveyard Optional", "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Hand | ChangeType$ Creature.YouOwn | Hidden$ True | Optional$ True", "Confirm"},
		{"known-origin Optional bounce", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Battlefield | Destination$ Hand | Optional$ True", "MayAsk: Bounce:from=Battlefield"},
		{"Attacking return", "SP$ ChangeZone | Cost$ B | Defined$ Self | Origin$ Graveyard | Destination$ Battlefield | Tapped$ True | Attacking$ True", "Attacking"},
		{"targeted Mandatory", "SP$ ChangeZone | Cost$ B | ValidTgts$ Card.OppOwn | Origin$ Graveyard | Destination$ Exile | Mandatory$ True", "Exile:"},
		{"DifferentNames search", "SP$ ChangeZone | Cost$ B | Origin$ Library | Destination$ Hand | ChangeType$ Land.Basic | ChangeNum$ 2 | DifferentNames$ True", "DifferentNames"},
		{"Self return", "SP$ ChangeZone | Cost$ B | ValidTgts$ Land.YouOwn | Origin$ Graveyard | Destination$ Battlefield | ThisDefinedAndTgts$ Self", "AlsoSelf"},
		{"Self shuffle", "SP$ ChangeZone | Cost$ B | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Library | Shuffle$ True | ThisDefinedAndTgts$ Self", "ToLibraryPair:"},
	} {
		t.Run("lowers "+c.name, func(t *testing.T) {
			got, ops := czLowered(t, c.root)
			if len(got) != 0 {
				t.Fatalf("refused: %q", got)
			}
			if !strings.Contains(strings.Join(ops, " "), c.want) {
				t.Errorf("ops %q lack %q", ops, c.want)
			}
		})
	}
}
