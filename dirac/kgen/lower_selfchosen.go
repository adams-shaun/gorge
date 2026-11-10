package kgen

import (
	"maps"
	"strings"
)

// selfAndChosenExile lowers a battlefield exile whose Defined$ names the
// source and the card the chain chose: `Defined$ Self & ChosenCard`
// (Fortune, Loyal Steed's "exile it and up to one creature that saddled it").
// gorge resolves the two referents in that order, Self then ChosenCard, and
// the kernel's Exile op moves one ref, so each referent lowers as its own
// move with the node's other parameters (RememberChanged$ appends each one
// that moved, in the same order). The chosen set holds at most the one card
// ChooseCard picked, none when the pick was declined.
func (lc *lowerCtx) selfAndChosenExile(ps *params) error {
	if ps.str("Origin") != "Battlefield" || ps.str("Destination") != "Exile" {
		return refuse("Defined$ %q beyond a battlefield exile", ps.m["Defined"])
	}
	if _, ok := ps.m["ValidTgts"]; ok {
		return refuse("Defined$ %q beside its own ValidTgts$", ps.m["Defined"])
	}
	ps.consume("Defined")
	each := newParams(maps.Clone(ps.m))
	for _, d := range []string{"Self", "ChosenCard"} {
		each.m["Defined"] = d
		if err := lc.changeZoneMove(each); err != nil {
			return err
		}
	}
	for k := range each.used {
		ps.used[k] = true
	}
	return nil
}

// isSelfAndChosen reports the `Self & ChosenCard` Defined$ spelling.
func isSelfAndChosen(ps *params) bool {
	return strings.Join(strings.Fields(ps.m["Defined"]), " ") == "Self & ChosenCard"
}
