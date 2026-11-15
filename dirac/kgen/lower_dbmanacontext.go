package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// A resolving DB$ Mana in an activated ability that is NOT a CR 605.1a mana
// ability: a loyalty ability, one that targets anywhere in its
// chain, or one whose cost or effect moves a library card. gorge's
// cards.IsManaAbilitySA classifies by the HEAD word (Mana/ManaReflected, or the
// ChooseColor chain), so every other AB with a DB$ Mana sub resolves on the
// stack and effMana adds the mana to the controller (ManaRecipients).
//
// Sacellum Godspeaker (AB$ Reveal | Cost$ T, no target, no library move) is a
// CR 605.1a mana ability by the rules, but gorge puts it on the stack
// (cards/ability_class.go L48): the kernel keeps refusing it by name.

const refuseDbManaMA = "DB$ Mana under a CR 605.1a mana ability gorge resolves on the stack (gorge cards/ability_class.go L48)"

// dbManaActivatedOK reports whether the activated ability being lowered is
// certainly no mana ability (loyalty, targeted, or library-moving), so its
// DB$ Mana resolves on the stack.
func (lc *lowerCtx) dbManaActivatedOK() bool {
	root := lc.chainRoot
	if root == nil || root.Kind != "AB" || root.API == "ChooseColor" {
		return false
	}
	if loyaltyMarked(root.Params["Planeswalker"]) {
		return true
	}
	if dbManaCostMovesLibrary(root.Params["Cost"]) {
		return true
	}
	for s, d := root, 0; s != nil && d < 16; s, d = s.Sub, d+1 {
		if strings.TrimSpace(s.Params["ValidTgts"]) != "" || strings.TrimSpace(s.Params["TargetType"]) != "" {
			return true
		}
		if dbManaNodeMovesLibrary(s) {
			return true
		}
	}
	return false
}

// dbManaCostMovesLibrary mirrors gorge costMovesLibraryCard.
func dbManaCostMovesLibrary(cost string) bool {
	for _, part := range strings.Fields(cost) {
		head, _, ok := strings.Cut(part, "<")
		if !ok {
			continue
		}
		switch head {
		case "Mill", "Draw", "ExileFromTop",
			"PutCardToLibFromHand", "PutCardToLibFromGrave", "PutCardToLibFromSameGrave", "PutCardToLibFromBattlefield":
			return true
		}
	}
	return false
}

// dbManaNodeMovesLibrary mirrors gorge effectMovesLibraryCard.
func dbManaNodeMovesLibrary(s *cards.SA) bool {
	switch s.API {
	case "Draw", "Mill", "Surveil", "Dig", "DigMultiple", "DigUntil", "Explore", "Connive",
		"Discover", "Learn", "Heist", "Seek", "ManifestDread":
		return true
	case "Manifest", "Cloak":
		z := strings.TrimSpace(s.Params["ChoiceZone"])
		return z == "" || dbManaZoneHasLibrary(z)
	case "ChangeZone", "ChangeZoneAll":
		return dbManaZoneHasLibrary(s.Params["Origin"]) || dbManaZoneHasLibrary(s.Params["Destination"])
	case "Play":
		return dbManaZoneHasLibrary(s.Params["ValidZone"])
	}
	return false
}

func dbManaZoneHasLibrary(list string) bool {
	for _, z := range strings.Split(list, ",") {
		if z = strings.TrimSpace(z); z == "Library" || z == "All" {
			return true
		}
	}
	return false
}
