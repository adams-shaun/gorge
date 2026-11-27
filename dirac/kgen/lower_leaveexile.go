package kgen

import (
	"maps"
	"strings"
)

// pumpLeaveExile lowers a Pump's LeaveBattlefield$ (Moira and Teshar, Dreams
// of the Dead; gorge registerLeaveExile from registerPumpEffects,
// engine/effects/combatfx.go:692, the same helper ChangeZone's
// LeaveBattlefield$ uses, engine/effects/leavebattlefield.go): the pumped
// object's promise, registered with the grant, that it is exiled instead of
// going anywhere else. The kernel's ExtLeaveExile record is keyed on the
// object and its incarnation and is ended by the departure itself, exactly
// the lifetime gorge gives a Duration$ Permanent grant (the move sweep ends
// both halves of the grant on the object's own move, leavebattlefield.go:440),
// so only that duration lowers: a shorter grant would have to end the
// promise early, which the record cannot. A leave to anything but exile is
// unhandled in gorge too (it Notes and registers nothing), and a set recipient
// is one record per object, which the op does not walk.
func (lc *lowerCtx) pumpLeaveExile(ps *params, obj lRef, zone int) error {
	v, ok := ps.m["LeaveBattlefield"]
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "Exile" || zone != zBattlefield ||
		strings.TrimSpace(ps.str("Duration")) != "Permanent" || !lc.pumpLeaveExileSingle(obj) {
		return refuse("Pump LeaveBattlefield$ %q (Duration$ %q, recipient %v, zone %d)", v, ps.str("Duration"), obj, zone)
	}
	ps.consume("LeaveBattlefield")
	lc.emit(lInstr{Op: "LeaveExile", Obj: obj, Note: "if it would leave the battlefield, it is exiled instead"})
	return nil
}

// pumpLeaveExileSingle reports whether obj names the one battlefield object
// the promise can ride (a Self, a single-object target slot, or a trigger
// object); a set of them is gorge's per-object loop, which the one record is
// not.
func (lc *lowerCtx) pumpLeaveExileSingle(obj lRef) bool {
	switch obj.Kind {
	case rTargetUnion, rEach, rEachPlayer, rEachOpp, rRemembered, rImprinted, rNone:
		return false
	}
	if obj.Kind == rTarget {
		return lc.singleObjectSlot(obj.N)
	}
	return true
}

// leaveExileMove lowers a ChangeZone with LeaveBattlefield$ (changeZoneMove's
// branch).
func (lc *lowerCtx) leaveExileMove(ps *params) error {
	// LeaveBattlefield$ Exile on a move to the battlefield (Isareth the
	// Awakener, From the Catacombs; gorge registerLeaveExile after the
	// object enters, zone_change.go): the move, then the exile-instead
	// promise on the one object it returned (ExtLeaveExile, the kernel's
	// zone_leaveexile.go).
	v := ps.m["LeaveBattlefield"]
	if strings.TrimSpace(v) != "Exile" || strings.TrimSpace(ps.m["Destination"]) != "Battlefield" {
		return refuse("LeaveBattlefield$ %q to %q", v, ps.m["Destination"])
	}
	ps.consume("LeaveBattlefield")
	ps.m = maps.Clone(ps.m) // the card's own map is the registry's, which gorge reads
	delete(ps.m, "LeaveBattlefield")
	start := len(lc.ab.Prog)
	if err := lc.changeZoneMove(ps); err != nil {
		return err
	}
	var obj lRef
	n := 0
	for i := start; i < len(lc.ab.Prog); i++ {
		switch in := &lc.ab.Prog[i]; in.Op {
		case "Return", "ReturnEx", "Reanimate":
			obj = in.Obj
			n++
		}
	}
	if n != 1 || obj.Kind == rNone {
		return refuse("LeaveBattlefield$ beside %d moves", n)
	}
	lc.emit(lInstr{Op: "LeaveExile", Obj: obj, Note: "if it would leave the battlefield, it is exiled instead"})
	return nil
}
