package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strings"
)

// objOrTarget resolves the object(s) a node acts on: its own target slot
// (ValidTgts$) or Defined$ (default dflt). A target slot reads TargetMin$/
// TargetMax$/TgtZone$/TargetUnique$ and friends.
func (lc *lowerCtx) objOrTarget(ps *params, dflt string, zone int) (lRef, error) {
	return lc.objOrTargetOpt(ps, dflt, zone, false)
}

// objOrTargetParent is objOrTarget for an effect that may restrict a card
// target to the graveyard of an earlier player target (gorge's
// TargetsWithDefinedController$ ParentTarget).
func (lc *lowerCtx) objOrTargetParent(ps *params, dflt string, zone int) (lRef, error) {
	return lc.objOrTargetOpt(ps, dflt, zone, true)
}

func (lc *lowerCtx) objOrTargetOpt(ps *params, dflt string, zone int, allowParent bool) (lRef, error) {
	if vt, ok := ps.get("ValidTgts"); ok {
		var gxZones []kop.Zone
		if lc.graveExileCard {
			var err error
			if vt, gxZones, err = graveExileSplit(vt); err != nil { // lower_graveexiletgt.go
				return lRef{}, err
			}
		}
		thanActive, vt, isThan := oathPlayerTarget(vt) // lower_thanactive.go
		f, err := lc.filter(vt)
		if err != nil {
			return lRef{}, err
		}
		t := lTarget{F: f, Zone: zone}
		if isThan {
			t.OppToActive, t.ThanActive = true, thanActive
			if err := lc.thanActiveChooser(ps, &t); err != nil {
				return lRef{}, err
			}
		}
		if lc.graveExileCard {
			if f.Players != 0 || len(f.Clauses) != len(gxZones) {
				return lRef{}, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ %q does not lower to one clause per alternative", vt)
			}
			t.GraveExileCard = true
			for _, z := range gxZones {
				t.ClauseZones = append(t.ClauseZones, int(z))
			}
		}
		if lc.permOrGraveCard {
			if f.Players != 0 {
				return lRef{}, refuse("ChangeZone Origin$ Battlefield,Graveyard with a player target")
			}
			for _, c := range f.Clauses {
				if c.Ctrl != whoAny || c.Owner != whoAny {
					return lRef{}, refuse("ChangeZone Origin$ Battlefield,Graveyard with a controller/owner filter")
				}
			}
			t.PermOrGraveCard = true
		}
		if z, ok := ps.get("TgtZone"); ok {
			t.Zone = zoneNames[z]
			if t.Zone == 0 {
				return lRef{}, refuse("TgtZone %q", z)
			}
		}
		if f.Players != 0 && len(f.Clauses) == 0 {
			t.Zone = zNone
		}
		if lc.otherOneEach(ps, f, t.Zone) { // the bounds are set here; forEachOpp checks the rest and sets the max
			t.Min, t.Max = cst(0), cst(1)
		} else {
			if v, ok := ps.get("TargetMin"); ok {
				if t.Min, err = lc.value(v); err != nil {
					return lRef{}, err
				}
			}
			if v, ok := ps.get("TargetMax"); ok {
				if t.Max, err = lc.value(v); err != nil {
					return lRef{}, err
				}
			}
			t.Min, t.Max = lc.oppCountBound(t.Min), lc.oppCountMax(ps, t.Max)
		}
		if allowParent {
			if v, ok := ps.get("TargetsWithDefinedController"); ok && v != "TriggeredCardController" && v != "TriggeredTarget" && v != "TriggeredAttackedTarget" { // those forms: lower_targetset.go
				if v != "ParentTarget" {
					return lRef{}, refuse("TargetsWithDefinedController$ %q", v)
				}
				parent := lc.lastTarget()
				if parent < 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget without an earlier target")
				}
				pt := &lc.ab.Targets[parent]
				if pt.F == nil || pt.F.Players == 0 || len(pt.F.Clauses) != 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent is not a player target")
				}
				// The kernel reads only the primary parent slot (e.tgt[parent-1]),
				// while gorge's filterParentControllerTargets uses every parent
				// target. A parent that can choose more than one player (or whose
				// target count is not a constant) would silently see only the
				// first. Min may be 0 (a declined parent names nobody: both
				// sides fail closed to no candidates) or 1.
				if !pt.Max.isConst() || pt.Max.N != 1 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent TargetMax$ %s is not 1", pt.Max)
				}
				if !pt.Min.isConst() || (pt.Min.N != 0 && pt.Min.N != 1) {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent TargetMin$ %s is not 0 or 1", pt.Min)
				}
				// The kernel's parentTargetOK accepts only a graveyard card
				// controlled by one player parent. gorge's
				// filterParentControllerTargets keeps object parents too,
				// and the effect's own zone may be wider, so read exactly
				// the supported shape.
				if f.Players != 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: target filter %q names players", vt)
				}
				if t.Zone != zGraveyard {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: target zone %s is not Graveyard", zoneLabel[t.Zone])
				}
				for _, c := range f.Clauses {
					if c.Owner != 0 || c.Ctrl != 0 {
						return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget with an owner/controller filter")
					}
				}
				t.ParentPlus1 = parent + 1
			}
		}
		if ps.is("TargetUnique", "True") {
			t.Unique = true
		}
		if ps.is("TargetsWithSameController", "True") {
			t.SameCtrl = true
		}
		if ps.is("TargetsWithDifferentControllers", "True") {
			t.DiffCtrl = true
		}
		if err := lc.tgtSetParams(ps, &t, allowParent); err != nil { // lower_targetset.go
			return lRef{}, err
		}
		if err := lc.forEachOpp(ps, f, &t); err != nil {
			return lRef{}, err
		}
		tr := lRef{Kind: rTarget, N: lc.addTarget(t)}
		if d, ok := ps.get("Defined"); ok {
			// gorge DefinedRef: a Defined$ selector wins over the chosen
			// targets as the effect's subject; the target is still
			// chosen (and checked on resolution) for the values and
			// later nodes that read it (Biomantic Mastery, Donate)
			if d == "Targeted" {
				lc.unionProbe("own-slot:Targeted")
				return tr, nil
			}
			return lc.defined(d)
		}
		return tr, nil
	}
	d := dflt
	if v, ok := ps.get("Defined"); ok {
		d = v
	}
	if d == "" {
		return lRef{}, refuse("no target and no Defined$")
	}
	if d == "Targeted" && len(lc.chainT) > 1 {
		return lc.targetUnion()
	}
	return lc.defined(d)
}

// targetedAndYou reads Defined$/TokenOwner$ TargetedAndYou (graveexile): gorge's
// alias for TargetedController & You (context.go:1834), deduplicated, the
// target player first then the controller. Only a target slot that is
// exactly ValidTgts$ Opponent is lowered: the dedupe can never merge the
// opponent with the controller (seat-exact), and any other filter (or a
// target-count rider) would change who receives the instruction.
func (lc *lowerCtx) targetedAndYou(ps *params, key string) (lRef, bool, error) {
	v, ok := ps.get(key)
	if !ok || strings.TrimSpace(v) != "TargetedAndYou" {
		return lRef{}, false, nil
	}
	vt, ok := ps.get("ValidTgts")
	if !ok || strings.TrimSpace(vt) != "Opponent" {
		return lRef{}, false, refuse("%s$ TargetedAndYou with ValidTgts$ %q", key, ps.str("ValidTgts"))
	}
	for _, k := range []string{"TargetMin", "TargetMax", "TgtZone", "TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers"} {
		if _, ok := ps.get(k); ok {
			return lRef{}, false, refuse("%s$ TargetedAndYou with %s$", key, k)
		}
	}
	f, err := lc.filter(vt)
	if err != nil {
		return lRef{}, false, err
	}
	if f.Players != plOpp || len(f.Clauses) != 0 {
		return lRef{}, false, refuse("%s$ TargetedAndYou target filter %q", key, vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zNone})
	ps.consume(key, "ValidTgts")
	return lRef{Kind: rTarget, N: t}, true, nil
}
