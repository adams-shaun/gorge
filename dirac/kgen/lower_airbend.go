package kgen

import "strings"

// api Airbend (CR 701.65a; gorge effects/airbend.go
// effAirbend) as OpExt ExtAirbend. The subject is an ordinary target slot
// (ValidTgts$ with TargetMin$/TargetMax$) or Defined$ Valid <filter>
// (every battlefield permanent matching it, read once at resolution; the
// sweep may end in .NotDefinedTargeted, which excludes the ability's one
// recorded target). Only battlefield permanents are airbent here: a
// spell-target filter (a TgtZone$, a head word with no permanent type)
// refuses, and so does every param beyond the subject.
func (lc *lowerCtx) airbend(ps *params) error {
	if _, ok := ps.m["TgtZone"]; ok {
		return refuse("Airbend with TgtZone$ (a spell or non-battlefield target)")
	}
	if d, ok := ps.m["Defined"]; ok {
		if _, t := ps.m["ValidTgts"]; t {
			return refuse("Airbend with both Defined$ and ValidTgts$")
		}
		d = strings.TrimSpace(d)
		spec, ok := strings.CutPrefix(d, "Valid ")
		if !ok || strings.Contains(spec, " & ") {
			return refuse("Airbend Defined$ %q", d)
		}
		ps.get("Defined")
		c := 0
		if rest, ex := strings.CutSuffix(spec, ".NotDefinedTargeted"); ex {
			if len(lc.chainT) != 1 {
				return refuse("Airbend .NotDefinedTargeted without exactly one earlier target")
			}
			spec, c = rest, lc.chainT[0]+1
		}
		f, err := lc.filter(spec)
		if err != nil {
			return err
		}
		if err := airbendFilter(f); err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Airbend", Obj: lRef{Kind: rNone}, F: f, C: c})
		return nil
	}
	if _, ok := ps.m["ValidTgts"]; !ok {
		return refuse("Airbend with no ValidTgts$ and no Defined$")
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("Airbend subject is not a target slot")
	}
	if f := lc.ab.Targets[obj.N].F; f == nil {
		return refuse("Airbend target without a filter")
	} else if err := airbendFilter(f); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Airbend", Obj: obj})
	return nil
}

// airbendFilter admits a filter only when every clause names permanent
// types alone (no player alternative, no card that could be a spell).
func airbendFilter(f *lFilter) error {
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("Airbend filter with a player alternative")
	}
	for _, c := range f.Clauses {
		if c.Types == 0 || c.Types&^tPermanent != 0 {
			return refuse("Airbend filter that can match a spell or a non-permanent card")
		}
	}
	return nil
}
