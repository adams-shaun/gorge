package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// exileCounterRider reports whether the dies-to-exile body w is the plain
// exile move of ReplacedCard that also leaves counters on the exiled card:
// WithCountersType$ on the move itself (Dauthi Voidwalker's VOID, Draugr
// Necromancer's ICE, Rayami's BLOOD), or RememberChanged$ True plus a
// PutCounter sub-ability on the Remembered card (Ravenloft Adventurer's HIT).
// A WithCountersAmount$ with no kind is not a counter rider, and a bare
// PutCounter on something else is not the rider chain. The bodies this
// predicate catches that exileBody does not lower (a chain with another shape,
// or WithCountersType$ plus a sub-ability) are refused by name rather than by
// the generic "Moved to Graveyard shape".
func exileCounterRider(w *cards.SA) bool {
	if w == nil || w.API != "ChangeZone" {
		return false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return false
	}
	switch bp.str("Origin") {
	case "", "All", "Battlefield":
	default:
		return false
	}
	if _, ok := bp.get("WithCountersType"); ok {
		return true
	}
	if _, ok := bp.get("WithCountersAmount"); ok {
		return false // an amount with no kind is not a counter rider
	}
	if w.Sub == nil || w.Sub.API != "PutCounter" || w.Sub.Params["Defined"] != "Remembered" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(bp.str("RememberChanged")), "True")
}

// exileCounterChainBody lowers the RememberChanged$ chain form of the
// dies-to-exile counter rider (Ravenloft Adventurer): the plain exile move of
// ReplacedCard with RememberChanged$ True, a PutCounter of one counter on
// Remembered, and a Cleanup of the remembered card. It is the same replacement
// as WithCountersType$ (the exiled card keeps the counter), so it returns the
// same kind and count exileBody's body-parameter form does. Any other
// parameter, a count other than one, a kind outside the exile-counter store or
// a longer chain is not this form.
func exileCounterChainBody(w *cards.SA) (ctr string, n uint8, ok bool) {
	pc := w.Sub
	if pc == nil || pc.API != "PutCounter" || pc.Sub == nil || pc.Sub.Sub != nil {
		return "", 0, false
	}
	cl := pc.Sub
	if cl.API != "Cleanup" {
		return "", 0, false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" ||
		!strings.EqualFold(bp.str("RememberChanged"), "True") {
		return "", 0, false
	}
	if o := bp.str("Origin"); o != "" && o != "All" && o != "Battlefield" {
		return "", 0, false
	}
	if h, has := bp.get("Hidden"); has && !strings.EqualFold(h, "True") {
		return "", 0, false
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden", "RememberChanged", "SubAbility")
	if len(bp.unread()) != 0 {
		return "", 0, false
	}
	pp := newParams(pc.Params)
	if pp.str("Defined") != "Remembered" {
		return "", 0, false
	}
	if v, has := pp.get("CounterNum"); has && strings.TrimSpace(v) != "1" {
		return "", 0, false
	}
	ctr = pp.str("CounterType")
	switch strings.ToUpper(ctr) {
	case "VOID", "ICE", "BLOOD", "HIT":
	default:
		return "", 0, false
	}
	pp.consume("Defined", "CounterType", "CounterNum", "SubAbility")
	if len(pp.unread()) != 0 {
		return "", 0, false
	}
	cp := newParams(cl.Params)
	if !strings.EqualFold(cp.str("ClearRemembered"), "True") {
		return "", 0, false
	}
	cp.consume("ClearRemembered")
	if len(cp.unread()) != 0 {
		return "", 0, false
	}
	return ctr, 1, true
}
