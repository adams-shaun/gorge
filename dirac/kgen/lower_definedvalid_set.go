package kgen

import (
	"maps"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// changeZoneDefinedValid lowers a ChangeZone whose Defined$ is the set
// spelling "Valid <filter>" (gorge's definedSpec battlefield set). The
// kernel's mass mover is the same op the explicit ChangeZoneAll API uses, so
// the node is re-read as an Origin$ Battlefield sweep over ChangeType$ <filter>
// (the pattern definedValidGraveyard uses for a graveyard set).
func (lc *lowerCtx) changeZoneDefinedValid(ps *params) (bool, error) {
	d := strings.TrimSpace(ps.str("Defined"))
	if !strings.HasPrefix(d, "Valid ") {
		return false, nil
	}
	if origin := strings.TrimSpace(ps.str("Origin")); origin != "Battlefield" {
		return true, refuse("ChangeZone Defined$ %q from Origin$ %q", d, origin)
	}
	cp := &params{m: maps.Clone(ps.m), used: map[string]bool{}}
	delete(cp.m, "Defined")
	cp.m["ChangeType"] = strings.TrimSpace(strings.TrimPrefix(d, "Valid "))
	err := lc.changeZoneAll(cp)
	for k := range cp.used {
		if k != "ChangeType" {
			ps.consume(k)
		}
	}
	ps.consume("Defined")
	return true, err
}

// unattachValid lowers Unattach Defined$ Valid <filter>: each battlefield
// permanent matching F that is still attached is detached, one pass per
// object (OpRepeatEach with RefEach), so the Unattached triggers fire per
// attachment in the subject order, as gorge's Defined walk does.
func (lc *lowerCtx) unattachValid(d string, ps *params) (bool, error) {
	d = strings.TrimSpace(d)
	if !strings.HasPrefix(d, "Valid ") {
		return false, nil
	}
	f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
	if err != nil {
		return true, err
	}
	if _, hasTarget := ps.get("ValidTgts"); hasTarget {
		// the node's own target is declared and chosen even though the
		// filter reads it through Card.targetedBy; objOrTarget would read
		// Defined$ back, so hand it a copy without the selector
		cp := &params{m: maps.Clone(ps.m), used: map[string]bool{}}
		delete(cp.m, "Defined")
		if _, err := lc.objOrTarget(cp, "", zBattlefield); err != nil {
			return true, err
		}
		for k := range cp.used {
			ps.consume(k)
		}
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("Unattach Defined$ %q: not a battlefield object set", d)
	}
	ps.consume("Defined")
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEachCard", F: f, C: kop.RepeatPlain, Skip: 1})
	lc.emit(lInstr{Op: "Unattach", Obj: lRef{Kind: rEach}})
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return true, nil
}

// exploreValid lowers Explore Defined$ Valid <filter>: each battlefield
// permanent matching F explores once (Hakbal's "each Merfolk creature you
// control explores"), one RepeatEach pass per object with RefEach.
func (lc *lowerCtx) exploreValid(ps *params, n lValue) (bool, error) {
	d := strings.TrimSpace(ps.str("Defined"))
	if !strings.HasPrefix(d, "Valid ") {
		return false, nil
	}
	if !n.isConst() || n.N != 1 {
		return true, refuse("Explore Defined$ %q with Num$ %v: only one explore per object lowers", d, n)
	}
	f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
	if err != nil {
		return true, err
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("Explore Defined$ %q: not a battlefield object set", d)
	}
	ps.consume("Defined")
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEachCard", F: f, C: kop.RepeatPlain, Skip: 1})
	lc.emit(lInstr{Op: "Explore", Obj: lRef{Kind: rEach}})
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return true, nil
}
