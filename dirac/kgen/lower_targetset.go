package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// target-SET constraints, one multi-select decision in
// gorge (effects/params/targets.go:294-309, decision.SetPropAdmits,
// rules/target_modes.go totalCMCCappedCandidates):
//
//   - MaxTotalTargetCMC$ N: the chosen cards' total mana value is at most N;
//   - TargetsWithDifferentNames$ True: no two chosen cards share a name;
//   - TargetsWithSameCreatureType$ True: the chosen objects share a creature
//     type (a changeling has every one);
//   - TargetsWithDefinedController$ TriggeredCardController: the candidates
//     are controlled by the dying trigger card's controller;
//   - TargetsWithDefinedController$ TriggeredTarget under a damage trigger
//     whose recipient is an opponent: the candidates are controlled by the
//     damaged opponent (tgtCtrlTriggeredTarget).
//
// Each reads only the shapes the kernel copies exactly; anything else
// refuses by name.

// tgtSetParams reads the four keys of ps into t (the slot objOrTargetOpt is
// building). allowParent is objOrTargetOpt's flag: the ParentTarget form of
// TargetsWithDefinedController$ is read there.
func (lc *lowerCtx) tgtSetParams(ps *params, t *lTarget, allowParent bool) error {
	f := t.F
	if v, ok := ps.get("TargetsWithDefinedController"); ok && !(allowParent && v == "ParentTarget") {
		if v == "TriggeredTarget" {
			if err := lc.tgtCtrlTriggeredTarget(t); err != nil {
				return err
			}
			f = t.F
		} else {
			if v != "TriggeredCardController" {
				return refuse("TargetsWithDefinedController$ %q", v)
			}
			tr := (*lTrigger)(nil)
			if lc.ab != nil {
				tr = lc.ab.Trigger
			}
			if tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "Dies" || tr.F == nil || tr.F.Players != 0 ||
				len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
				return refuse("TargetsWithDefinedController$ TriggeredCardController outside its own dies trigger")
			}
			if t.Zone != zGraveyard || f == nil || f.Players != 0 || len(f.Clauses) == 0 || t.ParentPlus1 != 0 {
				return refuse("TargetsWithDefinedController$ TriggeredCardController: not a graveyard card slot")
			}
			nf := *f
			nf.Clauses = append([]lClause(nil), f.Clauses...)
			for i := range nf.Clauses {
				c := &nf.Clauses[i]
				if c.Owner != 0 || c.Ctrl != 0 {
					return refuse("TargetsWithDefinedController$ TriggeredCardController with an owner/controller filter")
				}
				// a graveyard card is controlled by its owner: the dying card's
				// controller is the trigger's controller
				if err := c.setWho(&c.Owner, whoYou); err != nil {
					return err
				}
			}
			t.F = &nf
		}
	}
	nset := 0
	if ps.is("TargetsWithSameCreatureType", "True") {
		t.SharedCrType = true
		nset++
	}
	if ps.is("TargetsWithDifferentNames", "True") {
		t.DistinctName = true
		nset++
	}
	v, hasCap := ps.get("MaxTotalTargetCMC")
	if hasCap {
		nset++
	}
	if nset == 0 {
		return nil
	}
	if nset > 1 {
		return refuse("two target-set constraints on one slot")
	}
	if t.Unique || t.SameCtrl || t.DiffCtrl || t.ParentPlus1 != 0 {
		return refuse("target-set constraint beside TargetUnique/SameController/DifferentControllers/ParentTarget")
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("target-set constraint on a player target")
	}
	switch {
	case t.SharedCrType:
		if t.Zone != zGraveyard && t.Zone != zBattlefield {
			return refuse("TargetsWithSameCreatureType$ outside the battlefield and graveyard")
		}
		if !isConstN(t.Min, 2) || !isConstN(t.Max, 2) {
			return refuse("TargetsWithSameCreatureType$ with TargetMin$ %s TargetMax$ %s (want 2..2)", t.Min, t.Max)
		}
	case t.DistinctName:
		if t.Zone != zGraveyard || !isConstN(t.Min, 0) {
			return refuse("TargetsWithDifferentNames$ outside an optional graveyard slot")
		}
	default:
		if t.Zone != zGraveyard || !isConstN(t.Min, 0) {
			return refuse("MaxTotalTargetCMC$ outside an optional graveyard slot")
		}
		cv, err := lc.value(v)
		if err != nil {
			return err
		}
		if !cv.isConst() || cv.N < 0 || cv.N > 254 {
			return refuse("MaxTotalTargetCMC$ %s is not a literal 0..254", v)
		}
		t.MaxMV = cv.N + 1
	}
	return nil
}

// tgtSetSpec sets the target-set flags (and the cap) of ts from t.
func tgtSetSpec(t lTarget, ts *kop.TargetSpec) error {
	if !t.SharedCrType && !t.DistinctName && t.MaxMV == 0 {
		return nil
	}
	if t.Divided != 0 || t.AtRes {
		return fmt.Errorf("target-set constraint beside a divided or mid-resolution slot")
	}
	if t.SharedCrType {
		ts.TFlags |= kop.TgfSharedCrType
	}
	if t.DistinctName {
		ts.TFlags |= kop.TgfDistinctNames
	}
	if t.MaxMV != 0 {
		if t.MaxMV > 256 {
			return fmt.Errorf("MaxTotalTargetCMC$ above 255")
		}
		ts.TFlags |= kop.TgfMaxTotalMV
		ts.MaxMV = uint8(t.MaxMV - 1)
	}
	return nil
}

// tgtSetLIR is the LIR suffix of a target's set constraints (empty when none,
// so every other card's LIR is unchanged).
func tgtSetLIR(t lTarget) string {
	s := ""
	if t.SharedCrType {
		s += " sharedct=true"
	}
	if t.DistinctName {
		s += " distinctname=true"
	}
	if t.MaxMV != 0 {
		s += fmt.Sprintf(" maxmv=%d", t.MaxMV)
	}
	return s
}

// tgtCtrlTriggeredTarget is TargetsWithDefinedController$ TriggeredTarget
// gorge (rules/target_legal.go
// definedControllerPlayer) binds the referent to the trigger's target: the
// damaged player, or a damaged object's controller; the same predicate is
// rechecked on resolution (rules/stack.go, CR 608.2b). Only a DamageDone /
// DamageDealtOnce trigger whose recipient filter is exactly "an opponent"
// (ValidTarget$ Opponent) is read: in the kernel's two-player game the
// damaged recipient is then always the one opponent, so "controlled by
// TriggeredTarget" is exactly "controlled by an opponent", and the kernel's
// slot filter (rechecked on resolution) carries it. Anything else refuses.
func (lc *lowerCtx) tgtCtrlTriggeredTarget(t *lTarget) error {
	tr := (*lTrigger)(nil)
	if lc.ab != nil {
		tr = lc.ab.Trigger
	}
	if tr == nil || !lc.trigBody || lc.ab.Latent || (tr.Event != "DamageDone" && tr.Event != "DamageDealtOnce") {
		return refuse("TargetsWithDefinedController$ TriggeredTarget outside a damage trigger")
	}
	if tr.F2 == nil || tr.F2.Players != plOpp || len(tr.F2.Clauses) != 0 {
		return refuse("TargetsWithDefinedController$ TriggeredTarget: the damaged recipient is not an opponent")
	}
	f := t.F
	if t.Zone != zBattlefield || f == nil || f.Players != 0 || len(f.Clauses) == 0 || t.ParentPlus1 != 0 {
		return refuse("TargetsWithDefinedController$ TriggeredTarget: not a battlefield permanent slot")
	}
	nf := *f
	nf.Clauses = append([]lClause(nil), f.Clauses...)
	for i := range nf.Clauses {
		c := &nf.Clauses[i]
		if c.Owner != 0 || c.Ctrl != 0 {
			return refuse("TargetsWithDefinedController$ TriggeredTarget with an owner/controller filter")
		}
		if err := c.setWho(&c.Ctrl, whoOpp); err != nil {
			return err
		}
	}
	lc.twoSeat("TargetsWithDefinedController$ TriggeredTarget read as OppCtrl")
	t.F = &nf
	return nil
}
