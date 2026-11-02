package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// AlterAttribute Attributes$ Suspected (CR 701.60; gorge
// effects/misc.go effAlterAttribute) as OpExt ExtSuspect. The subject is the
// Defined$ object (Self, an absent Defined$ with no ValidTgts$, Targeted or an
// absent Defined$ beside ValidTgts$, Enchanted, Remembered, the trigger
// objects) or Defined$ Valid <filter>: every battlefield permanent matching
// it, read once at resolution (the Goad sweep shape). Activate$ False is the
// removal (C bit 0). Optional$ True is a may-election gated by the guard that
// gorge poses it under (alterAttributeWouldChange: some object would flip).
// RememberAltered$ and every other Activate$ value refuse.
func (lc *lowerCtx) alterSuspected(ps *params) error {
	c := 0
	if v, ok := ps.get("Activate"); ok {
		if strings.TrimSpace(v) != "False" {
			return refuse("AlterAttribute Suspected Activate$ %q", v)
		}
		c = 1
	}
	if _, ok := ps.get("RememberAltered"); ok {
		return refuse("AlterAttribute Suspected RememberAltered$")
	}
	var obj lRef
	var f *lFilter
	if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(strings.TrimSpace(d), "Valid") {
		d = strings.TrimSpace(d)
		if !strings.HasPrefix(d, "Valid ") || strings.Contains(d, " & ") {
			return refuse("AlterAttribute Suspected Defined$ %q", d)
		}
		ps.get("Defined")
		var err error
		if f, err = lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid "))); err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("AlterAttribute Suspected Defined$ %q: a player alternative", d)
		}
	} else {
		var err error
		if obj, err = lc.objOrTarget(ps, "Self", zBattlefield); err != nil {
			return err
		}
	}
	if err := lc.mayGate(ps, kop.MayGuardSuspect, lRef{Kind: rYou}); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Suspect", Obj: obj, F: f, C: c})
	return nil
}

// filterReadsSuspected reports whether f tests the suspected designation.
func filterReadsSuspected(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpSuspected {
			return true
		}
	}
	return false
}
