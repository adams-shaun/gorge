package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Blight<X> as a spell's own RaiseCost additional cost ("as an additional
// cost to cast this spell, blight X. X can't be greater than the greatest
// toughness among creatures you control", Soul Immolation; CR 701.68, 601.2b).
// The part is kop.CostBlight with Cost.N 0: the announced X is the number of
// -1/-1 counters put on the one creature the payer chooses. gorge's xAsk
// bounds every announced Blight<X> by the greatest toughness among the
// creatures the payer controls, whatever the spell's XMax$ says; the printed
// `Announce$ X | XMax$ GrTo` with SVar GrTo that very count is the only
// spelling lowered, so the kernel's built-in bound (cost_blightx.go) reads the
// card as printed. The spell prints no {X}: X is announced by the part alone.

// blightXGrTo is the SVar body of GrTo the kernel's bound implements.
const blightXGrTo = "Count$Valid Creature.YouCtrl$GreatestCardToughness"

// blightXBounded reports whether sa announces X bounded by the greatest
// toughness among the caster's creatures.
func (lc *lowerCtx) blightXBounded(sa *cards.SA) bool {
	p := sa.Params
	return strings.TrimSpace(p["Announce"]) == "X" && strings.TrimSpace(p["XMax"]) == "GrTo" &&
		strings.TrimSpace(lc.face.SVars["GrTo"]) == blightXGrTo
}

// blightXRaise parses cost, a lone Blight<X>, as a spell's own additional
// cost. ok is false for any other cost, and for a card whose spell ability
// does not announce X bounded by the greatest toughness, whose SVar X is not
// the announcement (Count$xPaid) or whose mana cost prints {X}.
func (lc *lowerCtx) blightXRaise(cost string) (lCost, bool) {
	if cost != "Blight<X>" || lc.out.Cost.X || strings.TrimSpace(lc.face.SVars["X"]) != "Count$xPaid" {
		return lCost{}, false
	}
	bounded := 0
	for _, sa := range lc.face.Abilities {
		if sa.Kind == "SP" && lc.blightXBounded(sa) {
			bounded++
		}
	}
	if bounded != 1 {
		return lCost{}, false
	}
	lc.blightXOK = true
	c, err := lc.cost(cost)
	lc.blightXOK = false
	if err != nil || !c.BlightX {
		return lCost{}, false
	}
	return c, true
}

// blightXBody consumes the XMax$ (and Announce$) of the spell ability sa of a
// card whose own RaiseCost is Blight<X>, when blightXRaise lowers it: the
// bound rides the cost part, not the body. Anything else keeps XMax$ unread
// and so refuses.
func (lc *lowerCtx) blightXBody(sa *cards.SA, ps *params) {
	if _, ok := ps.m["XMax"]; !ok || !lc.blightXBounded(sa) { // m, not get: get marks the key read
		return
	}
	for _, st := range lc.face.Statics {
		if st.Mode == "RaiseCost" && strings.TrimSpace(st.Params["Cost"]) == "Blight<X>" {
			ps.consume("Announce", "XMax")
			return
		}
	}
}
