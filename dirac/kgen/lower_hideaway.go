package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Hideaway (Collector's Cage; CR 702.75a). gorge kwHideaway expands
// K:Hideaway:N into an ETB trigger "DB$ Hideaway | Amount$ N" (effHideaway:
// the controller looks at the top N, exiles one face down linked to the
// source, bottoms the rest in a random order), and the card's own ability
// plays "Defined$ ExiledWith" (effPlay). Lowered as OpExt ExtHideaway: C 0
// the ETB (A the constant N), C 1 the Play; kernel kw_hideaway.go.

// hasHideaway reports whether the face carries K:Hideaway.
func (lc *lowerCtx) hasHideaway() bool {
	for _, k := range lc.face.Keywords {
		if strings.HasPrefix(k, "Hideaway") {
			return true
		}
	}
	return false
}

// hideaway lowers the keyword trigger's DB$ Hideaway.
func (lc *lowerCtx) hideaway(ps *params) error {
	if faceLayout(lc.card, lc.out.Token) != kop.FaceNone {
		return refuse("Hideaway on a multi-face card")
	}
	v, ok := ps.get("Amount")
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if !ok || err != nil || n < 1 || n > 24 {
		return refuse("Hideaway Amount$ %q", v)
	}
	lc.emit(lInstr{Op: "Hideaway", A: cst(n), Note: "look at the top cards, exile one face down, the rest to the bottom in a random order"})
	return nil
}

// hidePlay lowers the Play of the card exiled with the source (Defined$
// ExiledWith, Amount$ All, Optional$, WithoutManaCost$): the ConditionCheck
// SVar$ gate is the node's own SkipIf (nodeCondition).
func (lc *lowerCtx) hidePlay(ps *params, amount string) error {
	if _, ok := ps.m["UnlessCost"]; ok {
		// Widespread Thieving's pay-WUBRG gate: the queued cast and the unless
		// wrapper are unproven together
		return refuse("Play Defined$ ExiledWith with UnlessCost$")
	}
	if !lc.hasHideaway() {
		return refuse("Play Defined$ ExiledWith without Hideaway")
	}
	if amount != "All" {
		return refuse("Play Defined$ ExiledWith Amount$ %q", amount)
	}
	lc.emit(lInstr{Op: "Hideaway", A: cst(0), C: 1, Note: "may play the card exiled with this without paying its mana cost"})
	return nil
}
