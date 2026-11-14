package kgen

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lowerBloodthirst lowers a printed K:Bloodthirst:<N> (CR 702.54a: "If an
// opponent was dealt damage this turn, this permanent enters with N +1/+1
// counters on it.") as the replacement gorge synthesises for it
// (engine/rules/replacement_etb.go bloodthirstEntryMatch): a Moved
// Card.Self replacement whose body is an ETB PutCounter of N P1P1 counters,
// gated by CheckSVar$ Count$DamageOppsTakenThisTurn | SVarCompare$ GT0; for
// Bloodthirst X the count is the CounterNum$ and there is no gate. gorge has
// no cards-side expansion: the replacement is built at the entry, so the
// face carries no R: line for it and kgen builds the same one here. A param
// neither a positive literal nor X matches nothing in gorge: refused.
func (lc *lowerCtx) lowerBloodthirst(param string) error {
	body := &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
		"Defined":     "Self",
		"CounterType": "P1P1",
		"ETB":         "True",
	}}
	r := cards.Repl{Event: "Moved", Params: map[string]string{
		"Destination":       "Battlefield",
		"ValidCard":         "Card.Self",
		"ReplacementResult": "Updated",
		"Keyword":           "Bloodthirst",
		"KeywordLine":       "Bloodthirst:" + param,
	}, With: body}
	if param == "X" {
		body.Params["CounterNum"] = "Count$DamageOppsTakenThisTurn"
	} else {
		n, err := strconv.Atoi(param)
		if err != nil || n <= 0 {
			return refuse("keyword %q: bloodthirst amount", "Bloodthirst:"+param)
		}
		body.Params["CounterNum"] = strconv.Itoa(n)
		r.Params["CheckSVar"] = "Count$DamageOppsTakenThisTurn"
		r.Params["SVarCompare"] = "GT0"
	}
	return lc.lowerRepl(r)
}
