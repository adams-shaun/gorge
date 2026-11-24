package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// A resolving DB$ Mana in an activated ability that is NOT a CR 605.1a mana
// ability: a loyalty ability, one that targets anywhere in its
// chain, or one whose cost or effect moves a library card. Those lower as an
// ordinary resolving chain and effMana adds the mana to the controller
// (ManaRecipients).
//
// Gorge's classification of the remaining AB heads whose chain reaches a
// DB$ Mana, measured with cards.IsManaAbilitySA on the 2026-11-23 corpus:
// gorge's classifier (engine/cards/ability_class.go) takes
// ANY activated AB whose SubAbility$ chain reaches a DB$ Mana
// (ManaChainProduction), minus the exclusions (a Planeswalker$ loyalty ability,
// a target anywhere in the chain, a cost or effect that moves a library card).
// So the ChooseColor chains (Nyx Lotus, Rhystic Cave, Nykthos, Shrine to Nyx's
// second ability, Foraging Wickermaw), the Reveal chains (Sacellum Godspeaker,
// Metalworker), Jack-in-the-Mox (RollDice) and Mana Screw (FlipCoin) are all
// CR 605.1a mana abilities gorge resolves OFF the stack
// (engine/rules/mana_activation.go answerNestedChooseColor), not on it: the
// message below predates that classifier's chain arm. An AB$ Mana head
// declaring ValidTgts$ (Radiant Lotus) is no mana ability and lowers through
// dbManaActivatedOK.
//
// The kernel's chosen-colour arm lowers Foraging Wickermaw's shape: a
// generic-only fee, no {T}, ActivationLimit$ 1 and (optionally) a
// chosen-colour Animate rider (lower_choosecolmana.go). It rides the
// planner-deferred mana-rider route, as gorge's planner defers a source with a
// SubAbility$ chain (pay.PaymentPlanAbilityTier source:last_resort), so its
// census never prices it. The {T} + Chosen devotion forms (Nyx Lotus, Nykthos,
// Rhystic Cave) are still refused here until the base arm lands (ticket
// t18e3-dbmana-choosecol-base).

const refuseDbManaMA = "DB$ Mana under a CR 605.1a mana ability gorge resolves on the stack (gorge cards/ability_class.go L48)"

// dbManaActivatedOK reports whether the activated ability being lowered is
// certainly no mana ability (loyalty, targeted, or library-moving), so its
// DB$ Mana resolves on the stack.
func (lc *lowerCtx) dbManaActivatedOK() bool {
	root := lc.chainRoot
	if root == nil || root.Kind != "AB" || root.API == "ChooseColor" {
		return false
	}
	return dbManaExcluded(root)
}

// dbManaExcluded reports whether the activated chain rooted at root has one of
// the reasons gorge's classifier takes it for no mana ability: a loyalty
// marker, a target anywhere in the chain, or a library-moving cost or effect.
func dbManaExcluded(root *cards.SA) bool {
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
