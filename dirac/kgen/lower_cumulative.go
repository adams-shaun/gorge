package kgen

import (
	"strconv"
	"strings"
)

// cumulativeUpkeep lowers the body of gorge's cumulative upkeep expansion
// (CR 702.24a; engine/cards/kw_cumulativeupkeep.go: an upkeep trigger of
// ValidPlayer$ You, body DB$ CumulativeUpkeep | Cost$ <cost>, Mystic Remora).
// gorge's startCumulativeUpkeep puts an age counter on the source, then asks
// its controller to pay the cost once per age counter or sacrifice it
// (cumulative_pay / cumulative_sac, over the same resPay machinery as a
// trigger's own cost). The program is
//
//	Cumulative                  the age counter (skips the rest when the source is gone)
//	UnlessPaid You Mana n*AGE   the pay-or-decline ask, skipping the sacrifice when paid
//	Sacrifice self              a decline
//
// Only a plain generic cost lowers ({1}, {2}, ...): a coloured, hybrid,
// Phyrexian or {X} cost, a life, and the non-mana action costs of
// parseCumulativeAction (sacrifice a creature, discard, add a counter, ...)
// stay refused by name.
func (lc *lowerCtx) cumulativeUpkeep(ps *params) error {
	raw, ok := ps.get("Cost")
	if !ok {
		return refuse("CumulativeUpkeep without Cost$")
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 15 {
		return refuse("cumulative upkeep cost %q (only a generic mana cost lowers)", raw)
	}
	if lc.ab.Kind != akTriggered {
		return refuse("cumulative upkeep outside a trigger")
	}
	lc.emit(lInstr{Op: "Cumulative", C: 2, Note: "age counter; the payment and the sacrifice are skipped if the source is gone"})
	lc.emit(lInstr{Op: "UnlessPaid", Obj: lRef{Kind: rYou}, CS: "Mana", Skip: 1,
		A:    lValue{Kind: vCountersOn, Ref: lRef{Kind: rSelf}, S: "AGE", Mul: n},
		B:    cst(0),
		Note: "pay the cost once per age counter, else sacrifice"})
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}
