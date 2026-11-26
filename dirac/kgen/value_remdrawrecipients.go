package kgen

import "strings"

// remDrawRecipientsBody is the count head of Cut a Deal's SVar X,
// PlayerCountOpponents$HasPropertyIsRememberedOrController: "a card for each
// opponent who drew a card this way".
const remDrawRecipientsBody = "HasPropertyIsRememberedOrController"

// remDrawRecipientsValue lowers it (gorge playerCount's
// HasPropertyIsRememberedOrController arm, engine/effects/count_player.go):
// the opponents that are the live controller of an entry of the resolution's
// remembered set. The set is seeded by the chain's one RememberDrawn$ Draw of
// each opponent (the first node of Cut a Deal), so an opponent counts when
// that draw put a card in their hand. A drawing player's hand cards are their
// own, so the entry's controller is the drawer, and every card the draw moved
// (AllReplaced) names the same player.
//
// A two-player game has one opponent, so the count is 1 when the set holds a
// card and 0 when it is empty (a draw from an empty library, or a replacement
// that moved nothing, remembers nothing). The reduction is exact only there,
// so the card is marked two-seat; any other shape of the remembered set (no
// writer, several writers, or a writer that is not that draw) refuses.
func (lc *lowerCtx) remDrawRecipientsValue() (lValue, error) {
	if lc.ab == nil {
		return lValue{}, refuse("PlayerCountOpponents$%s outside an ability", remDrawRecipientsBody)
	}
	writers, drawn := 0, false
	for _, in := range lc.ab.Prog {
		if kremWrites(in) {
			writers++
			drawn = in.Op == "Draw" && in.Obj.Kind == rEachOpp && strings.Contains(in.CS, "Remember")
		}
	}
	if writers != 1 || !drawn {
		return lValue{}, refuse("PlayerCountOpponents$%s beside a remembered set other than one opponents' RememberDrawn$ Draw", remDrawRecipientsBody)
	}
	lc.twoSeat("the opponents who drew a remembered card counted as one (PlayerCountOpponents$" + remDrawRecipientsBody + ")")
	return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{
		{Kind: vRememberedCount, S: "ctx"},
		{Kind: vBranches, Sub: []lValue{cst(1), cst(0)}},
	}}, nil
}
