package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// triggerExcludedOrigins lowers ExcludedOrigins$ on a self enters trigger
// ("Name Sticker" Goblin: "when this creature enters from anywhere other than
// a graveyard or exile"; gorge trigmatch ZoneChangeMatches: the move must not
// originate in a listed zone). The kernel's ETB event carries no origin, but
// its entered ledger records every battlefield add with the zone it came from
// (value_enteredledger.go), so each excluded zone lowers to the condition
// "Count$ThisTurnEntered_Battlefield_from_<Zone>_Card.Self is 0" restricted to
// the source's latest entry (kop.EnteredLastFrom), the move entering now: an
// earlier graveyard entry this turn, since bounced and recast, is not read. Only the graveyard and exile words
// lower; any other list, or a line that is not a self enters trigger,
// refuses by name.
func (lc *lowerCtx) triggerExcludedOrigins(t cards.Trigger, tp *params) ([]lCond, error) {
	raw, ok := tp.get("ExcludedOrigins")
	if !ok {
		return nil, nil
	}
	if t.Mode != "ChangesZone" || strings.TrimSpace(t.Params["Destination"]) != "Battlefield" ||
		strings.TrimSpace(t.Params["ValidCard"]) != "Card.Self" {
		return nil, refuse("ExcludedOrigins$ %q beside a trigger that is not a self enters", raw)
	}
	var conds []lCond
	for _, w := range strings.Split(raw, ",") {
		w = strings.TrimSpace(w)
		if w != "Graveyard" && w != "Exile" {
			return nil, refuse("ExcludedOrigins$ %q", raw)
		}
		v, err := lc.value("Count$ThisTurnEntered_Battlefield_from_" + w + "_Card.Self")
		if err != nil {
			return nil, err
		}
		v.From = kop.EnteredLastFrom(v.From) // only the entry that is entering now
		conds = append(conds, lCond{Kind: cValue, A: v, Op: opEQ, B: cst(0)})
	}
	return conds, nil
}

// exclOriginsSplit joins the origin conditions to a trigger's other
// conditions. gorge reads ExcludedOrigins$ as the event matches
// (trigmatch.ZoneChangeMatches), never again at resolution, while the
// IsPresent$ clauses are intervening-ifs read at both: a Cloudshift in
// response puts the source's exile entry on the ledger, and the trigger still
// resolves. So the whole conjunction gates the trigger (CheckOnTriggerOnly)
// and the other conditions guard the body, which runs at resolution. A trigger
// already fire-only, or with a body guard of its own, refuses.
func exclOriginsSplit(excl, conds []lCond, guard lCond, fireOnly bool) ([]lCond, lCond, bool, error) {
	if fireOnly || guard.Kind != cNone {
		return nil, lCond{}, false, refuse("ExcludedOrigins$ beside a fire-only or trailing-if gate")
	}
	all := append(append([]lCond(nil), conds...), excl...)
	for i, c := range conds {
		if i == 0 {
			guard = c
		} else {
			guard = andCond(guard, c)
		}
	}
	return all, guard, true, nil
}
