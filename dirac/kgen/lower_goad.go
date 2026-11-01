package kgen

import "strings"

// api Goad (CR 701.15; gorge effects/misc.go effGoad) as OpGoad.
// The default duration (until the goader's next turn) only: Duration$
// AsLongAsInPlay / AsLongAsControl, NoLonger$ and RememberGoaded$ stay
// unread and refuse. The subject is a target (ValidTgts$), a Defined$
// object (Targeted, Self, ...), or Defined$ Valid <filter>: every
// battlefield permanent matching it, read once at resolution.
func (lc *lowerCtx) goad(ps *params) error {
	if strings.TrimSpace(ps.m["Duration"]) == "UntilYourNextTurn" {
		ps.get("Duration")
	}
	if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(strings.TrimSpace(d), "Valid") {
		d = strings.TrimSpace(d)
		if !strings.HasPrefix(d, "Valid ") || strings.Contains(d, " & ") {
			return refuse("Goad Defined$ %q", d)
		}
		ps.get("Defined")
		f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
		if err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("Goad Defined$ %q: a player alternative", d)
		}
		lc.emit(lInstr{Op: "Goad", Obj: lRef{Kind: rNone}, F: f})
		return nil
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Goad", Obj: obj})
	return nil
}
