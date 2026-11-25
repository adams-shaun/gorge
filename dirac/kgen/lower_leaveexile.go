package kgen

import (
	"maps"
	"strings"
)

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
