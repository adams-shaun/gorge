package kgen

import (
	"strconv"
	"strings"
)

// playRemNotChosen is Plargg and Nassari's population: the remembered cards
// but the one the opponent chose (gorge DefinedSpec's Remembered with the
// !ChosenCard filter).
const playRemNotChosen = "Remembered.!ChosenCard"

// playUpToOK reports whether a Play's Amount$ (neither 1 nor All) lowers onto
// CastFree's UpTo bit: gorge effPlay's one ask with Max N, the picks begun in
// answer order (engine/effects/play.go). A literal N >= 2 over the remembered
// exile (Doom Reigns Supreme's "up to two spells") or over Plargg and
// Nassari's remembered-but-chosen set. A counted Amount$ lowers over the two
// populations whose walk the kernel already matches to gorge effPlay's: the
// graveyard filter population (Chandra Ablaze's Amount$ ChandraX, the count
// of red instants and sorceries you own) and the remembered library (Talent
// of the Telepath's spell-mastery Amount$ X). gorge prices the count through
// NumResolved and asks with that Max; a priced count below one offers nothing
// (the kernel's count-0 break beside the empty pool, interp_gen.go) and an
// unpriced one stays refused by lower_zone.go's lc.value read. A copy cast
// keeps the refusal.
func playUpToOK(ps *params, amount string, copyCard bool) bool {
	if copyCard {
		return false
	}
	def, valid, vzone := ps.m["Defined"], ps.m["Valid"], ps.m["ValidZone"]
	if n, err := strconv.Atoi(amount); err == nil {
		return n >= 2 &&
			(def == "" && vzone == "Exile" && strings.Contains(valid, "IsRemembered") ||
				def == playRemNotChosen && valid == "" && vzone == "")
	}
	if _, ok := ps.m["ForgetPlayed"]; ok {
		return false
	}
	if _, ok := ps.m["ReplaceGraveyard"]; ok {
		return false
	}
	if _, ok := ps.m["ValidTgts"]; ok {
		return false
	}
	switch vzone {
	case "Graveyard":
		// the P4 filter population over every alive seat's graveyard, the
		// ownership clause carried by the filter (lower_zone.go's Hand/
		// Graveyard arm); Chandra Ablaze's Instant.Red+YouCtrl,...
		return def == "" && (strings.Contains(valid, "YouOwn") || strings.Contains(valid, "YouCtrl"))
	case "Library":
		// the P1l remembered cards in the resolving controller's library
		// (Talent of the Telepath's revealed seven stay in the revealed
		// seat's slice, which gorge effPlay's private-zone walk does not
		// read; the kernel walks the controller's slice likewise)
		return def == "" && valid == "Card.IsRemembered"
	}
	return false
}

// playDefinedPop lowers the Defined$ populations play does not read inline:
// Plargg and Nassari's Remembered.!ChosenCard, the remembered cards where they
// lie with the chosen card filtered out. Every other Defined$ refuses by name;
// Harness the Storm's Defined$ Targeted is the chain's one TargetOnly
// graveyard slot, a paid cast of that card.
func (lc *lowerCtx) playDefinedPop(ps *params, def, saExpr string, emit func(lRef, int, string) error) error {
	_, hasValid := ps.m["Valid"]
	_, hasVZ := ps.m["ValidZone"]
	_, forget := ps.m["ForgetPlayed"]
	_, replace := ps.m["ReplaceGraveyard"]
	if def == "Targeted" && !hasValid && !hasVZ && !forget && !replace && len(lc.chainT) == 1 {
		return emit(lRef{Kind: rTarget, N: lc.lastTarget()}, zGraveyard, saExpr)
	}
	if def != playRemNotChosen || hasValid || hasVZ || forget || replace {
		return refuse("Play Defined$ %q", def)
	}
	expr, err := andFilterExpr("Card.nonChosenCard", saExpr)
	if err != nil {
		return err
	}
	return emit(lRef{Kind: rRemembered}, lc.playRememberedZone(), expr)
}
