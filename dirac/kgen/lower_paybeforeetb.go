package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// payBeforeETB lowers Mox Diamond's pay-before-entering replacement: "If this
// would enter, you may discard a land card instead. If you do, put it onto the
// battlefield. If you don't, put it into its owner's graveyard." gorge's body
// is the exact chain
//
//	Discard (Optional, DiscardValid Land, TgtChoose, RememberDiscarded)
//	  -> ChangeZone ReplacedCard to the graveyard while the remembered count is 0
//	  -> ChangeZone ReplacedCard to the battlefield while the count is 1
//	  -> Cleanup ClearRemembered
//
// and the lowering is the Discard alone: the kernel runs it as the permanent
// spell resolves, then enters the card when it remembered a discard and
// graveyards it otherwise (dirac/repl_paybeforeetb.go). Any other chain keeps
// its refusal (false, nil).
func (lc *lowerCtx) payBeforeETB(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if rp.str("Destination") != "Battlefield" || rp.str("ValidCard") != "Card.Self" || w == nil || w.API != "Discard" {
		return false, nil
	}
	n1, n2, n3 := w.Sub, (*cards.SA)(nil), (*cards.SA)(nil)
	var n4 *cards.SA
	if n1 != nil {
		n2 = n1.Sub
	}
	if n2 != nil {
		n3 = n2.Sub
	}
	if n3 != nil {
		n4 = n3.Sub
	}
	gate := func(cmp string) map[string]string {
		return map[string]string{"Origin": "All", "Defined": "ReplacedCard", "ConditionCheckSVar": "X", "ConditionSVarCompare": cmp, "SubAbility": "x"}
	}
	toGrave, toField := gate("EQ0"), gate("EQ1")
	toGrave["Destination"], toField["Destination"] = "Graveyard", "Battlefield"
	if !nodeIs(n1, "ChangeZone", toGrave) || !nodeIs(n2, "ChangeZone", toField) ||
		!nodeIs(n3, "Cleanup", map[string]string{"ClearRemembered": "True"}) ||
		n4 != nil || strings.TrimSpace(lc.face.SVars["X"]) != "Remembered$Amount" {
		return false, nil
	}
	dp := newParams(w.Params)
	dp.consume("SubAbility")
	if dp.str("Optional") != "True" || dp.str("Mode") != "TgtChoose" || dp.str("DiscardValid") != "Land" || dp.str("RememberDiscarded") != "True" {
		return false, nil
	}
	rp.consume("Destination", "ValidCard")
	ab := &lAbility{Kind: akLatent}
	if err := lc.withAbility(ab, func() error { return lc.discard(dp) }); err != nil {
		return true, err
	}
	if left := dp.unread(); len(left) > 0 {
		return true, refuse("pay-before-ETB discard unread params %v", left)
	}
	rl.Prog = ab.Prog
	rl.Kind = "PayBeforeETB"
	return true, nil
}

// nodeIs: the chain node is DB$ api whose parameters are exactly want's
// (SubAbility only when want names it; descriptive keys ignored), with no
// further check on its Sub link (the caller walks the chain).
func nodeIs(n *cards.SA, api string, want map[string]string) bool {
	if n == nil || n.API != api {
		return false
	}
	c := 0
	for k, v := range n.Params {
		if ignoredParams[k] {
			continue
		}
		if k == "SubAbility" {
			if _, ok := want[k]; !ok {
				return false
			}
			c++
			continue
		}
		if w, ok := want[k]; !ok || strings.TrimSpace(w) != strings.TrimSpace(v) {
			return false
		}
		c++
	}
	return c == len(want)
}
