package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stackManaAbility reports whether an AB$ Mana is no mana ability by CR
// 605.1a because its cost or chain moves a library card (cards.IsManaAbilitySA's
// library gate: the four Mill<1> cost carriers, Chromatic Sphere, the five
// Eggs). gorge resolves it on the stack: the ability is offered at priority
// as an ordinary activated ability ("ability"), pays its cost as one (a mana
// ability could be activated mid-payment; this one cannot), and on resolution
// adds its mana and runs the rest of the chain. finishMana lowers it as a
// loyalty ability's mana: an activated ability whose program is the mana,
// then the chain.
//
// A cost that draws, exiles from the top of a library or puts a card into one
// is left to the existing refusals (dbManaCostMovesLibraryBeyondMill): the
// kernel's activation-cost payment settles a Cost.Mill only (mana_window.go
// payAbilityCosts). A target anywhere in the chain is left to the existing
// refusals too.
func (lc *lowerCtx) stackManaAbility(sa *cards.SA, ab *lAbility) bool {
	if sa == nil || ab.Loyalty || ab.Kind != akMana || sa.Kind != "AB" || sa.API != "Mana" || cards.IsManaAbilitySA(sa) {
		return false
	}
	if dbManaCostMovesLibraryBeyondMill(sa.Params["Cost"]) {
		return false
	}
	for s, d := sa, 0; s != nil && d < 16; s, d = s.Sub, d+1 {
		if strings.TrimSpace(s.Params["ValidTgts"]) != "" || strings.TrimSpace(s.Params["TargetType"]) != "" {
			return false
		}
	}
	return true
}

// dbManaCostMovesLibraryBeyondMill mirrors gorge costMovesLibraryCard for the
// library-moving cost parts whose payment the kernel's activation costs do not
// model (only a Cost.Mill is, mana_window.go payAbilityCosts): a cost that
// draws, exiles from the top of a library or puts a card into one keeps the
// ability refused.
func dbManaCostMovesLibraryBeyondMill(cost string) bool {
	for _, part := range strings.Fields(cost) {
		head, _, ok := strings.Cut(part, "<")
		if !ok {
			continue
		}
		switch head {
		case "Draw", "ExileFromTop",
			"PutCardToLibFromHand", "PutCardToLibFromGrave", "PutCardToLibFromSameGrave", "PutCardToLibFromBattlefield":
			return true
		}
	}
	return false
}
