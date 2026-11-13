package kgen

import "strings"

// czallDefinedScope reads a ChangeZoneAll's Defined$ player scope with no
// ValidTgts$ of its own (Go Blank's "Then exile all cards from that player's
// graveyard": Defined$ ParentTarget). gorge changeZoneAllPlayers sweeps only
// the players Defined$ resolves to (zone_changeall.go; Forge's
// getTargetPlayers on the non-targeting path); the kernel's sweep reads one
// player off in.Obj (execChangeZoneAll's playerRef). Lowered: You, and the
// chain's one player target slot (TargetMax$ 1) under Targeted /
// ParentTarget / ParentTargeted / TargetedPlayer. Any other spelling (each
// opponent, an object's owner, a trigger referent) refuses by name.
func (lc *lowerCtx) czallDefinedScope(ps *params, in *lInstr) error {
	v, ok := ps.get("Defined")
	if !ok {
		return nil
	}
	switch v = strings.TrimSpace(v); v {
	case "You":
		in.Obj = lRef{Kind: rYou}
		in.Note = "only the controller's zone"
		return nil
	case "Targeted", "ParentTarget", "ParentTargeted", "TargetedPlayer":
		t := lc.lastTarget()
		if v != "Targeted" && v != "TargetedPlayer" {
			t = lc.parentLastTarget() // never the node's own slot (lower_owntarget.go)
		}
		if t < 0 || len(lc.chainT) > 1 && v == "Targeted" {
			return refuse("ChangeZoneAll Defined$ %s without one target slot", v)
		}
		pt := &lc.ab.Targets[t]
		if pt.F == nil || pt.F.Players == 0 || len(pt.F.Clauses) != 0 {
			return refuse("ChangeZoneAll Defined$ %s over a non-player target", v)
		}
		if !pt.Max.isConst() || pt.Max.N != 1 || !pt.Min.isConst() || pt.Min.N != 1 {
			// gorge sweeps every chosen player, and every player when none
			// was chosen (its Note); the kernel reads exactly one
			return refuse("ChangeZoneAll Defined$ %s over a target of %s..%s players", v, pt.Min, pt.Max)
		}
		in.Obj = lRef{Kind: rTarget, N: t}
		in.Note = "only the target player's zone"
		return nil
	}
	return refuse("ChangeZoneAll Defined$ %q", v)
}
