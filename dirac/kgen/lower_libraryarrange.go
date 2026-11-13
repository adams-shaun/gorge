package kgen

import "strings"

// libraryArrangeRefusal refuses a ChangeZoneAll or ChangeZone whose cards can
// land in a library position together: gorge poses the owner's CR 401.4
// arrangement ("may arrange them in any order", effects.libraryPlacementOrder,
// a KArrange the replay records) when one effect puts two or more cards of one
// owner at the top or bottom, and the kernel still places them in its fixed
// order. Lowering the ask is a later change; until then the card is refused by
// name here rather than diverging. The ask is skipped when the order is random
// or the library is shuffled afterwards (RandomOrder$, Shuffle$ True), so those
// shapes still lower, and a Hidden$ mover never asks.
//
// ChangeZoneAll sweeps a whole zone, so any sweep to a library may carry two
// cards. A ChangeZone carries several only through a multi-target pick
// (TargetMax$ past 1) or a Defined$ set other than Self. It reads its params
// with peek: this is a guard, not a consumer, and must not alter what the
// census counts as read.
func libraryArrangeRefusal(ps *params, api string) error {
	if ps.peek("Destination") != "Library" || ps.peek("RandomOrder") == "True" {
		return nil
	}
	if ps.peek("Shuffle") == "True" && ps.peek("NoShuffle") != "True" {
		return nil
	}
	switch api {
	case "ChangeZoneAll":
		switch strings.TrimSpace(ps.peek("LibraryPosition")) {
		case "", "0", "-1": // the positions gorge's sweep arranges
			return refuse("ChangeZoneAll to a library position: the CR 401.4 arrangement of several cards is not lowered")
		}
	case "ChangeZone":
		if ps.peek("Hidden") == "True" {
			return nil
		}
		if tm := ps.peek("TargetMax"); tm != "" && tm != "1" {
			return refuse("ChangeZone TargetMax$ %q to a library: the CR 401.4 arrangement of several cards is not lowered", tm)
		}
		if d := ps.peek("Defined"); d != "" && d != "Self" {
			return refuse("ChangeZone Defined$ %q to a library: the CR 401.4 arrangement of several cards is not lowered", d)
		}
	}
	return nil
}
