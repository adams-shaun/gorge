package kgen

import "strings"

// czAltDest lowers ChangeZone's conditional alternate destination: Forge's
// DestAltSVar$ names a condition (an SVar, usually Count$Kicked or
// Count$wasCastFromGraveyard) whose value is compared under
// DestAltSVarCompare$ (default GE1). A "MANDATORY " prefix takes
// DestinationAlternative$ directly when it holds; without the prefix the
// controller chooses (Emeria Shepherd), and that shape lowers through
// OpAskDestAlt (czAltDestAsk below).
//
// gorge evaluates it once, at the start of the resolution:
// changeZoneAltDestination (engine/effects/zone_change.go:60-117) is called
// by changeZonePrelude (zone_change.go:134-176, the call at :168), and
// effChangeZone (:351-, prelude at :353) runs that prelude before every path
// (the library search, the hidden pick and the target/object move). The
// destination is fixed before the search pick, so the lowering is a two-arm
// branch with the same layout as branch:
//
//	SkipIf cond (skip when false: over ALT and the jump); ALT...;
//	SkipIf never (jump over PRIMARY); PRIMARY...
//
// The first arm declares the node's target slot; the second reads it through
// Defined$ Targeted, so the ability has one target slot, as in gorge.
func (lc *lowerCtx) czAltDest(ps *params) error {
	raw, _ := ps.get("DestAltSVar")
	ask := !strings.HasPrefix(raw, "MANDATORY ")
	sv := strings.TrimSpace(strings.TrimPrefix(raw, "MANDATORY "))
	if sv == "" {
		return refuse("DestAltSVar$ without a condition")
	}
	altText, _ := ps.get("DestinationAlternative")
	altText = strings.TrimSpace(altText)
	alt, err := zoneOf(ps, "DestinationAlternative", zNone)
	if err != nil {
		return err
	}
	if alt == zNone {
		return refuse("DestAltSVar$ without a known DestinationAlternative$")
	}
	// LibraryPositionAlternative$ and AlternativeDecider$ are the "owner
	// chooses top or bottom" shape, not a condition.
	if v, ok := ps.get("LibraryPositionAlternative"); ok {
		return refuse("DestAltSVar$ with LibraryPositionAlternative$ %q", v)
	}
	if v, ok := ps.get("AlternativeDecider"); ok {
		return refuse("DestAltSVar$ with AlternativeDecider$ %q", v)
	}
	if ask {
		if err := czAskShape(ps, raw, alt); err != nil {
			return err
		}
	}
	v, err := lc.value(sv)
	if err != nil {
		return err
	}
	op, b := opGE, cst(1)
	if cmp, ok := ps.get("DestAltSVarCompare"); ok && strings.TrimSpace(cmp) != "" {
		op, b, err = lc.cmpSpec(strings.TrimSpace(cmp))
		if err != nil {
			return refuse("DestAltSVarCompare$ %q", cmp)
		}
	}
	primary, err := zoneOf(ps, "Destination", zNone)
	if err != nil {
		return err
	}
	if primary == zNone {
		return refuse("ChangeZone without Destination")
	}
	// Both arms start from the same parameters, minus the three keys this
	// function owns. Sharing ps.used keeps chain's unread-params check exact:
	// a key neither arm reads still refuses, and a key either arm reads is
	// marked read.
	armMap := func() map[string]string {
		m := make(map[string]string, len(ps.m))
		for k, val := range ps.m {
			m[k] = val
		}
		delete(m, "DestAltSVar")
		delete(m, "DestAltSVarCompare")
		delete(m, "DestinationAlternative")
		return m
	}
	altM := armMap()
	altM["Destination"] = altText
	if alt != zBattlefield {
		// Tapped$ applies only to a battlefield entry; a hand (or any other)
		// destination ignores it in gorge.
		delete(altM, "Tapped")
	}
	primM := armMap()
	if primary != zBattlefield {
		delete(primM, "Tapped")
	}
	// The first arm declared the node's target slot. The second arm must
	// read that slot instead of declaring a second one: drop the target
	// declaration keys and name the slot with Defined$ Targeted. A defined
	// selector other than Targeted already names the subject in the first
	// arm, so keep it (objOrTarget's Defined$ wins over its own targets).
	secondM := primM
	firstM := altM
	if ask {
		firstM, secondM = primM, altM
	}
	nt := len(lc.ab.Targets)
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: v, Op: op, B: b}})
	askAt := -1
	if ask {
		// the condition false: over the ask, the primary destination alone
		lc.ab.Prog[at].Skip = 1
		askAt = len(lc.ab.Prog)
		lc.emit(lInstr{Op: "AskDestAlt", Note: "the controller chooses the primary or the alternate destination"})
	}
	if err := lc.changeZone(&params{m: firstM, used: ps.used}); err != nil {
		return err
	}
	if !ask {
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at // over ALT and the jump
	}
	if _, ok := ps.m["ValidTgts"]; ok {
		declared := len(lc.ab.Targets) > nt
		if !declared {
			return refuse("DestAltSVar$ first arm does not declare the ValidTgts$ slot")
		}
		if len(lc.chainT) != 1 {
			return refuse("DestAltSVar$ with %d target slots in the chain", len(lc.chainT))
		}
		for _, k := range []string{"ValidTgts", "TgtPrompt", "TargetMin", "TargetMax",
			"TgtZone", "TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers"} {
			delete(secondM, k)
		}
		if d, ok := secondM["Defined"]; !ok || strings.TrimSpace(d) == "" || strings.TrimSpace(d) == "Targeted" {
			secondM["Defined"] = "Targeted"
		}
	}
	jt := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, Note: "jump over the primary destination"})
	if err := lc.changeZone(&params{m: secondM, used: ps.used}); err != nil {
		return err
	}
	lc.ab.Prog[jt].Skip = len(lc.ab.Prog) - jt - 1
	if ask {
		lc.ab.Prog[askAt].Skip = jt - askAt // the alternate: over PRIMARY and the jump
		if lc.ab.Prog[askAt].Skip > 255 {
			return refuse("DestAltSVar$ choice over %d instructions", lc.ab.Prog[askAt].Skip)
		}
	}
	return nil
}

// czAskShape holds the destination choice to the shape gorge poses it for:
// changeZoneAltDestination asks in effChangeZone's prelude, except for a
// library search, whose ask comes after the pick (searchDefersDestination).
// The two zones must differ, and the other alternative-destination keys
// stay refused.
func czAskShape(ps *params, raw string, alt int) error {
	origin := ps.peek("Origin")
	if origin == "" || strings.Contains(origin, "Library") {
		return refuse("DestAltSVar$ %q without MANDATORY from Origin$ %q (a destination choice)", raw, origin)
	}
	if d, _ := zoneOf(ps, "Destination", zNone); d == alt {
		return refuse("DestAltSVar$ %q choice between the same zone", raw)
	}
	return nil
}
