package kgen

import (
	"strconv"
	"strings"
)

// raiseWaterbendN reads a RaiseCost Cost$ of Waterbend<N> (N in 1..255) and
// reports whether it is that shape; `Waterbend<X>` is the variable form.
func raiseWaterbendN(cost string) (n int, fixed, isX bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(cost), "Waterbend<")
	if !ok {
		return 0, false, false
	}
	rest, ok = strings.CutSuffix(rest, ">")
	if !ok {
		return 0, false, false
	}
	if rest == "X" {
		return 0, false, true
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 || n > 255 {
		return 0, false, false
	}
	return n, true, false
}

// raiseSelfWaterbend lowers a spell's own `S:Mode$ RaiseCost | ValidCard$
// Card.Self | Activator$ You | Type$ Spell | Cost$ Waterbend<N> | EffectZone$
// All` ("as an additional cost to cast this spell, waterbend {N}", CR 701.67a,
// 601.2b/f-h). gorge folds the static into the pending cast (composeRaiseCost:
// N more generic, payable by tapping artifacts and creatures). The lowering is
// card data, not a static: CardDef.SpellWaterbend (kernel act_waterbend.go).
// handled is true when the static is a Waterbend cost (st is then dropped);
// every other Cost$ falls through to raiseSelfBehold. The variable
// Waterbend<X> form is refused: its pick cap and X fold are not modelled.
func (lc *lowerCtx) raiseSelfWaterbend(sp *params, st *lStatic) (bool, error) {
	cost, ok := sp.m["Cost"]
	if !ok || sp.m["Amount"] != "" {
		return false, nil
	}
	n, fixed, isX := raiseWaterbendN(cost)
	if !fixed && !isX {
		return false, nil
	}
	sp.consume("Cost")
	cost = strings.TrimSpace(cost)
	if isX {
		return true, refuse("spell waterbend %q: a variable amount is not modelled", cost)
	}
	if sp.str("ValidCard") != "Card.Self" || sp.str("Activator") != "You" || sp.str("Type") != "Spell" || sp.str("EffectZone") != "All" {
		return true, refuse("RaiseCost Cost$ %q: not the card's own spell cost", cost)
	}
	if _, has := sp.m["Condition"]; has {
		return true, refuse("RaiseCost Cost$ %q with a Condition$", cost)
	}
	raises := 0
	for _, s := range lc.face.Statics {
		if s.Mode == "RaiseCost" {
			raises++
		}
	}
	o := lc.out
	if raises != 1 || o.SpellWaterbend != 0 || o.BeholdN != 0 || o.SpellCost != nil || o.AltAddCost != nil || o.OptCost != nil ||
		o.Convoke || o.Improvise || o.Cost.X {
		return true, refuse("RaiseCost Cost$ %q beside another raise or an additional cost", cost)
	}
	o.SpellWaterbend = n
	st.Kind = "" // card data only: lowerStatic drops a kind-less static
	return true, nil
}
