package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// shuffleNamedCard lowers one synthetic spell whose root targets a player
// (when the case needs a parent) and whose ChangeZone sub-ability carries the
// shuffle shape.
func shuffleNamedCard(t *testing.T, root, sub map[string]string) *lCard {
	t.Helper()
	line := subAbilityLine(sub)
	if v, ok := sub["ShuffleNonMandatory"]; ok {
		line += " | ShuffleNonMandatory$ " + v
	}
	svars := map[string]string{"DBChangeZone": line}
	rootSA := &cards.SA{Kind: "SP", API: "Pump", Params: root, Sub: cards.ResolveSVar(svars, "DBChangeZone")}
	if rootSA.Sub == nil {
		t.Fatal("synthetic sub-ability did not resolve")
	}
	face := &cards.Face{
		Name:      "Synthetic",
		ManaCost:  "G",
		Types:     []string{"Sorcery"},
		SVars:     svars,
		Abilities: []*cards.SA{rootSA},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	if len(lc.Refusals) != 0 {
		t.Fatalf("refused: %v", lc.Refusals)
	}
	return lc
}

// TestShuffleIntoNamedLibrary pins the W operand the multi-object
// (OpMayShuffleMove) and single-object (OpToLibrary) shuffle lowerings carry:
// the parent player for TargetsWithDefinedController$ ParentTarget, the
// controller when every ValidTgts$ alternative is YouCtrl/YouOwn, and none
// for the owner-defined Card set. The kernel shuffles W's library when the
// target set is empty (CR 701.24d).
func TestShuffleIntoNamedLibrary(t *testing.T) {
	player := map[string]string{"ValidTgts": "Player"}
	grave := func(valid, min, max string) map[string]string {
		return map[string]string{
			"Origin": "Graveyard", "Destination": "Library", "Shuffle": "True",
			"ValidTgts": valid, "TargetMin": min, "TargetMax": max,
		}
	}

	t.Run("multi parent target", func(t *testing.T) {
		sub := grave("Card", "0", "4")
		sub["TargetsWithDefinedController"] = "ParentTarget"
		lc := shuffleNamedCard(t, player, sub)
		in := findShuffleOp(t, lc, "MayShuffleMove")
		if in.W != (lRef{Kind: rTarget, N: 0}) {
			t.Errorf("W = %v, want the parent player target slot: the parent player names the library", in.W)
		}
	})

	t.Run("multi your library", func(t *testing.T) {
		lc := shuffleNamedCard(t, map[string]string{}, grave("Creature.YouCtrl", "0", "4"))
		in := findShuffleOp(t, lc, "MayShuffleMove")
		if in.W.Kind != rYou {
			t.Errorf("W = %v, want the controller: the YouCtrl filter names your library", in.W)
		}
	})

	t.Run("multi owner defined", func(t *testing.T) {
		lc := shuffleNamedCard(t, map[string]string{}, grave("Card", "0", "3"))
		in := findShuffleOp(t, lc, "MayShuffleMove")
		if in.W.Kind != rNone {
			t.Errorf("W = %v, want no named library: the owner-defined Card set names none", in.W)
		}
	})

	t.Run("multi may election", func(t *testing.T) {
		sub := grave("Card.YouCtrl", "0", "4")
		sub["ShuffleNonMandatory"] = "True"
		lc := shuffleNamedCard(t, map[string]string{}, sub)
		in := findShuffleOp(t, lc, "MayShuffleMove")
		if in.Obj.Kind != rYou {
			t.Fatalf("Obj = %v, want the controller asker: the fixture is the may form", in.Obj)
		}
		if in.W.Kind != rNone {
			t.Errorf("W = %v, want none: the may election does nothing for an empty set, so it names no library", in.W)
		}
	})

	t.Run("single parent target", func(t *testing.T) {
		sub := grave("Card", "1", "1")
		sub["TargetsWithDefinedController"] = "ParentTarget"
		lc := shuffleNamedCard(t, player, sub)
		in := findShuffleOp(t, lc, "ToLibrary")
		if in.W != (lRef{Kind: rTarget, N: 0}) {
			t.Errorf("W = %v, want the parent player target slot: the parent player names the library", in.W)
		}
	})

	t.Run("single your library", func(t *testing.T) {
		lc := shuffleNamedCard(t, map[string]string{}, grave("Creature.YouCtrl", "1", "1"))
		in := findShuffleOp(t, lc, "ToLibrary")
		if in.W.Kind != rYou {
			t.Errorf("W = %v, want the controller: the YouCtrl filter names your library", in.W)
		}
	})

	t.Run("single owner defined", func(t *testing.T) {
		lc := shuffleNamedCard(t, map[string]string{}, grave("Card", "1", "1"))
		in := findShuffleOp(t, lc, "ToLibrary")
		if in.W.Kind != rNone {
			t.Errorf("W = %v, want no named library: the owner-defined Card set names none", in.W)
		}
	})
}

// findShuffleOp returns the card's one emitted op, asserting it is want.
func findShuffleOp(t *testing.T, lc *lCard, want string) lInstr {
	t.Helper()
	var got []lInstr
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op == want {
				got = append(got, in)
			}
		}
	}
	if len(got) != 1 {
		t.Fatalf("built %d %s instructions, want 1", len(got), want)
	}
	return got[0]
}
