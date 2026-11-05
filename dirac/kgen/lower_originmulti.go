package kgen

import "strings"

// originBattlefieldGrave reads a ChangeZone whose Origin$ names the
// battlefield and the graveyard (The Spot, Living Portal: "exile up to one
// target nonland permanent and up to one target nonland permanent card from a
// graveyard", the two targets chosen by the Pump nodes before it).
//
// gorge effChangeZone moves each Defined$ object that is in the Origin$ set
// (changeZoneMoveSkip: OriginMask.Has(o.Zone)); a target that left both zones
// is skipped, as the kernel skips a target that is no longer legal. The
// kernel's targeted exile moves the object from whichever zone it is in
// (moveObj), so the set names no instruction operand: only a Defined$
// Targeted exile reads it, and the origin is lowered as unrestricted.
// Every other shape (a destination but exile, a hidden pick) refuses by name.
func (lc *lowerCtx) originBattlefieldGrave(ps *params) error {
	ps.get("Origin") // read
	if _, ok := ps.get("ValidTgts"); ok {
		// one slot across both zones (Aurelia's Vindicator): gorge's
		// changeZoneTargetZones names the battlefield and the graveyard
		if ps.str("Destination") != "Exile" || ps.str("Defined") != "" {
			return refuse("ChangeZone Origin$ Battlefield,Graveyard with ValidTgts$ (a target across both zones)")
		}
		for _, k := range []string{"ChangeType", "Hidden", "Optional", "Mandatory", "DefinedPlayer", "Chooser", "GainControl", "LibraryPosition", "TgtZone", "TargetType"} {
			if _, ok := ps.get(k); ok {
				return refuse("ChangeZone Origin$ Battlefield,Graveyard with ValidTgts$ and %s$", k)
			}
		}
		lc.permOrGraveCard = true
		return nil
	}
	switch {
	case ps.str("Destination") != "Exile":
		return refuse("ChangeZone Origin$ Battlefield,Graveyard to %q", ps.str("Destination"))
	case ps.str("Defined") != "Targeted":
		return refuse("ChangeZone Origin$ Battlefield,Graveyard without Defined$ Targeted")
	}
	for _, k := range []string{"ChangeType", "Hidden", "Optional", "Mandatory", "DefinedPlayer", "Chooser", "GainControl", "LibraryPosition"} {
		if _, ok := ps.get(k); ok {
			return refuse("ChangeZone Origin$ Battlefield,Graveyard with %s$", k)
		}
	}
	return nil
}

// isOriginBattlefieldGrave reports an Origin$ of exactly the battlefield and
// the graveyard, in either order.
func isOriginBattlefieldGrave(o string) bool { return isBattlefieldGraveSet(strings.TrimSpace(o)) }
