package kgen

import (
	"strings"
)

func (lc *lowerCtx) pumpAll(ps *params) error {
	if spec, ok := targetedOtherSpec(ps.m["ValidCards"]); ok {
		return lc.pumpAllOtherThanTarget(ps, spec) // lower_pumpallother.go
	}
	f, err := lc.filter(ps.str("ValidCards"))
	if err != nil {
		return err
	}
	if done, err := lc.sharedPumpAll(ps, f); done || err != nil {
		return err // lower_sharedpumpall.go
	}
	if kw, ok := ps.m["KW"]; ok && strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(kw), "HIDDEN ")) == skipNextUntapKW {
		return lc.skipNextUntapAll(ps, f) // Sleep
	}
	if dur, ok := ps.get("Duration"); ok {
		return lc.pumpAllDuration(ps, f, dur)
	}
	scope, err := lc.pumpAllScope(ps)
	if err != nil {
		return err
	}
	if err := lc.pumpAllRemember(ps, f, scope); err != nil { // lower_rememberwritersmass.go
		return err
	}
	att, def := cst(0), cst(0)
	if a, ok := ps.get("NumAtt"); ok {
		if att, err = lc.value(a); err != nil {
			return err
		}
	}
	if d, ok := ps.get("NumDef"); ok {
		if def, err = lc.value(d); err != nil {
			return err
		}
	}
	in := lInstr{Op: "PumpAll", F: f, A: att, B: def, Obj: scope}
	if scope.Kind != rNone {
		in.Note = "only the target player's permanents"
	}
	mustBlock, eachColor, marks := false, false, 0
	var afterlives []int
	var attackKws []string
	var protAll *lOverride
	var landwalk *lFilter
	if kw, ok := ps.get("KW"); ok {
		kw, mustBlock = splitKWSentence(kw, mustBeBlockedKW)
		if mustBlock && scope.Kind != rNone {
			return refuse("PumpAll KW$ must be blocked if able with a ValidTgts$ player scope")
		}
		kw, eachColor = splitKWSentence(kw, protEachColorKW)
		if eachColor && scope.Kind != rNone {
			return refuse("PumpAll KW$ protection from each color with a ValidTgts$ player scope")
		}
		kw, marks = splitKwMarks(kw) // lower_kwmark.go
		if marks != 0 && scope.Kind != rNone {
			return refuse("PumpAll KW$ %s with a ValidTgts$ player scope", kwMarkNames(marks))
		}
		if kw, protAll, err = lc.splitProtSpecAll(kw); err != nil { // lower_protgrant.go
			return err
		}
		if protAll != nil && scope.Kind != rNone {
			return refuse("PumpAll KW$ Protection:<spec> with a ValidTgts$ player scope")
		}
		if kw, landwalk, err = lc.splitLandwalkAll(kw, scope); err != nil { // lower_landwalkall.go
			return err
		}
		if kw != "" {
			if kw, afterlives, err = splitAfterlifeGrant(kw); err != nil {
				return err
			}
			if len(afterlives) > 0 && scope.Kind != rNone {
				return refuse("PumpAll KW$ Afterlife with a ValidTgts$ player scope")
			}
			if kw, attackKws, err = splitAttackKwGrant(kw); err != nil { // lower_attackkwgrant.go
				return err
			}
			if err := refuseEffectReturnKw(attackKws); err != nil {
				return err
			}
			if len(attackKws) > 0 && scope.Kind != rNone {
				return refuse("PumpAll KW$ %s with a ValidTgts$ player scope", attackKws[0])
			}
		}
		if kw != "" {
			ks, err := pumpKeywords(kw)
			if err != nil {
				return err
			}
			in.CS = strings.Join(ks, "&")
		}
	}
	_, hasAtt := ps.m["NumAtt"]
	_, hasDef := ps.m["NumDef"]
	if err := pumpAllHighKw(&in, hasAtt || hasDef); err != nil { // lower_pumpallhighkw.go
		return err
	}
	if !(mustBlock || eachColor || marks != 0 || len(afterlives) > 0 || len(attackKws) > 0 || protAll != nil || landwalk != nil) || in.CS != "" || hasAtt || hasDef {
		lc.emit(in)
	}
	if landwalk != nil {
		lc.grantLandwalkAll(f, landwalk)
	}
	if err := lc.grantAfterlifeAll(f, afterlives); err != nil { // lower_afterlifegrant.go
		return err
	}
	if err := lc.grantAttackKwAll(f, attackKws); err != nil { // lower_attackkwgrant.go
		return err
	}
	if protAll != nil {
		// CR 702.16 + 611.2c: the permanents matching ValidCards$ as this
		// resolves have protection from the quality until end of turn (Spare
		// from Evil): an end-of-turn override record swept over them
		lc.out.overrides = append(lc.out.overrides, protAll)
		lc.emit(lInstr{Op: "OverrideAll", F: f, CS: "EOT", Note: overrideString(protAll), C: len(lc.out.overrides) - 1})
	}
	if eachColor {
		// CR 702.16 + 611.2c: the permanents matching ValidCards$ as this
		// resolves have protection from each color until end of turn (gorge
		// sourceHasQuality's "each color": any coloured source). The same
		// marks as a chosen-colour grant, one per colour (ExtColorGrant C
		// cgProt|cgProtAll; Akroma's Will)
		lc.emit(lInstr{Op: "ColorGrant", F: f, A: cst(0), C: 12, Note: "until end of turn: each matching permanent has protection from each color"})
	}
	if mustBlock {
		lc.emit(lInstr{Op: "MustBeBlocked", F: f}) // every permanent matching ValidCards$
	}
	if marks != 0 {
		lc.emit(lInstr{Op: "KwMark", F: f, C: marks, Note: kwMarkNames(marks) + " until end of turn"}) // every permanent matching ValidCards$
	}
	return nil
}

// pumpAllScope lowers PumpAll ValidTgts$ "creatures target player controls"
// (gorge targetPlayerKindScope: only the targeted player's permanents; an
// unresolvable scope sweeps nothing): the player target slot rides Obj. No
// ValidTgts$ is the zero scope.
func (lc *lowerCtx) pumpAllScope(ps *params) (lRef, error) {
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return lRef{}, nil
	}
	pf, err := lc.filter(vt)
	if err != nil {
		return lRef{}, err
	}
	if len(pf.Clauses) != 0 || pf.Players == 0 {
		return lRef{}, refuse("PumpAll ValidTgts$ %q: not a player target", vt)
	}
	return lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf})}, nil
}

// pumpAllDuration lowers PumpAll Duration$ UntilYourNextTurn as an
// OpOverrideAll over every permanent matching ValidCards$, as gorge does
// (CR 611.2c): AddKw for a keyword grant, PTDelta for a NumAtt$/NumDef$
// pump (a constant, read once; the kernel adds it after counters until the
// effect owner's next turn). Any other Duration$ refuses.
func (lc *lowerCtx) pumpAllDuration(ps *params, f *lFilter, dur string) error {
	if dur != "UntilYourNextTurn" {
		return refuse("PumpAll Duration$ %q", dur)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("PumpAll Duration$ UntilYourNextTurn with ValidTgts$")
	}
	o := &lOverride{}
	a, hasA := ps.get("NumAtt")
	d, hasD := ps.get("NumDef")
	if hasA || hasD {
		att, def := cst(0), cst(0)
		var err error
		if hasA {
			if att, err = lc.value(a); err != nil {
				return err
			}
		}
		if hasD {
			if def, err = lc.value(d); err != nil {
				return err
			}
		}
		if !att.isConst() || !def.isConst() {
			return refuse("PumpAll Duration$ UntilYourNextTurn with a computed P/T pump")
		}
		o.PTDelta, o.PVal, o.TVal = true, att, def
	}
	if kw, ok := ps.get("KW"); ok {
		ks, err := pumpKeywords(kw)
		if err != nil {
			return err
		}
		o.AddKw = ks
	} else if !o.PTDelta {
		return refuse("PumpAll Duration$ UntilYourNextTurn without KW$")
	}
	in := lInstr{Op: "OverrideAll", F: f, CS: "UntilYourNextTurn", Note: overrideString(o)}
	lc.out.overrides = append(lc.out.overrides, o)
	in.C = len(lc.out.overrides) - 1
	lc.emit(in)
	return nil
}

// skipNextUntapAll lowers a PumpAll whose KW$ is exactly CR 611.2b's
// next-untap-step sentence (gorge registerPumpEffects stamps
// Object.CantUntapNextStep on each swept permanent at grant, whatever the
// Duration$) as OpSkipNextUntap over the filter. Defined$ Targeted scopes
// the sweep to the chain's player target (gorge cr-sleep-defined-targeted:
// targetPlayerKindScope inherits the parent's player target; Sleep). A P/T
// change, another keyword or any other scope refuses.
func (lc *lowerCtx) skipNextUntapAll(ps *params, f *lFilter) error {
	ps.consume("KW")
	_, na := ps.m["NumAtt"]
	_, nd := ps.m["NumDef"]
	if na || nd {
		return refuse("next-untap-step sweep beside a P/T change")
	}
	if d, ok := ps.get("Duration"); ok && d != "Permanent" {
		return refuse("next-untap-step sweep Duration %q", d)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("next-untap-step sweep with ValidTgts$")
	}
	var scope lRef
	if d, ok := ps.get("Defined"); ok {
		t := lc.lastTarget()
		if d != "Targeted" || t < 0 {
			return refuse("next-untap-step sweep Defined$ %q", d)
		}
		if pf := lc.ab.Targets[t].F; pf == nil || len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("next-untap-step sweep Defined$ Targeted of an object target")
		}
		scope = lRef{Kind: rTarget, N: t}
	}
	lc.emit(lInstr{Op: "SkipNextUntap", Obj: scope, F: f, Note: "each matching permanent (the target player's)"})
	return nil
}
