package kgen

import "strings"

// cdaFlash lowers the conditional "this spell has flash" characteristic-
// defining static (CR 604.3, 613.3) to the carrier's own CastWithFlash
// static, the one kernel CardDef.SelfFlash reads (dirac/cast_selfflash.go):
//
//	S:Mode$ Continuous | CharacteristicDefining$ True | AddKeyword$ Flash |
//	  IsPresent$ Desert.YouCtrl | Description$ CARDNAME has flash as long as ...
//
//	Colossal Rattlewurm  IsPresent$ Desert.YouCtrl
//	Crashing Tide        IsPresent$ Merfolk.YouCtrl
//	Supernatural Rescue  IsPresent$ Spirit.YouCtrl
//	Take for a Ride      CheckSVar$ Count$CommittedCrimeThisTurn
//	Graveyard Shift      CheckSVar$ X (X: Count$ValidGraveyard Card.YouOwn$DifferentCardManaCost)
//
// gorge grants the flash from hand through chars.StaticCDAFlashAdmits while
// the gate holds (characters.go); the kernel's selfFlash evaluates the same
// gate for the caster with src = NoObj (evalCtx{src: NoObj, ctrl: p}), so the
// IsPresent$/PresentZone$/CheckSVar$ condition staticCond already lowered onto
// st.Cond must never read the source. done reports that the static was taken
// (lowered or refused by name); a shape that is not this CDA is left to
// continuous.
func (lc *lowerCtx) cdaFlash(sp *params, st *lStatic) (done bool, err error) {
	if !sp.is("CharacteristicDefining", "True") || !sp.is("AddKeyword", "Flash") {
		return false, nil
	}
	// The exact shape: the gate keys staticCond reads, plus the three keys
	// consumed below. Anything else (SetPower$/SetToughness$ is the P/T CDA,
	// SetColor$/AddType$ the colour/type one, a Condition$ the continuous
	// gate) stays with continuous, which refuses or lowers it as before.
	for k := range sp.m {
		switch k {
		case "Mode", "CharacteristicDefining", "AddKeyword", "Affected",
			"IsPresent", "PresentZone", "PresentCompare", "CheckSVar", "SVarCompare":
		default:
			if !ignoredParams[k] {
				return false, nil
			}
		}
	}
	aff := sp.str("Affected")
	if aff != "" && aff != "Card.Self" {
		return false, nil
	}
	if lc.hasFlash() {
		return false, refuse("CDA flash on a card with flash")
	}
	if err := lc.flashPresentShape(sp.m, "CDA flash"); err != nil {
		return false, err
	}
	if _, ok := sp.m["CheckSVar"]; ok && condReadsSource(st.Cond) {
		return false, refuse("CDA flash CheckSVar reads the source")
	}
	f, err := lc.filter("Card.Self")
	if err != nil {
		return false, err
	}
	st.Kind = "CastWithFlash"
	st.F = f
	lc.out.SelfFlash = true
	// The same gate also grants the keyword on the battlefield: gorge's CDA
	// applies in every zone, so the permanent has Flash while the gate holds
	// (chars.StaticCDAFlashAdmits). A Continuous grant beside the hand cast
	// gives the kernel the same derived keyword (kwGen reads it under Cond2
	// with the permanent as source), so the lockstep's keyword projection
	// agrees; the hand cast above is still the only cast-timing reader.
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "Continuous", F: f, Kw: []string{"Flash"}, Cond: st.Cond})
	sp.consume("CharacteristicDefining", "AddKeyword", "Affected")
	return true, nil
}

// flashPresentShape checks the IsPresent$/PresentZone$ gate a self flash
// grant may read. The gate is evaluated for the caster with the carrier still
// in hand (no battlefield source), so a source-relative IsPresent$ term
// (Permanent.YouCtrl+hasKeywordFlash) is refused: the cast-time gate cannot
// read a battlefield permanent's keyword.
func (lc *lowerCtx) flashPresentShape(params map[string]string, what string) error {
	if ip, ok := params["IsPresent"]; ok && !selfFlashPresent.MatchString(strings.TrimSpace(ip)) {
		return refuse("%s IsPresent %q", what, ip)
	}
	if _, ok := params["IsPresent"]; !ok {
		if _, ok := params["PresentZone"]; ok {
			return refuse("%s PresentZone$ without IsPresent$", what)
		}
	}
	return nil
}

// nonSourceRef: a ref the caster's gate context can resolve (the controller
// is set); every other ref needs a source, a trigger or a resolution.
func nonSourceRef(r lRef) bool {
	return r.Kind != rNone && r.Kind != rYou
}

// filterReadsSource reports whether f tests the reading source (or a
// resolution/trigger object the cast-time gate has none of).
func filterReadsSource(f *lFilter) bool {
	if f == nil {
		return false
	}
	const srcFlags = fSelf | fOther | fEquippedBy | fEnchantedBy | fChosenType | fChosenColor |
		fRemembered | fNotRemembered | fChosenCard | fNotChosenCard | fAttachedToParentTarget |
		fTargetedBy | fNamedAsSource | fSharesColorWithOtherRemembered | fBlockedBySrc |
		fDamagedBy | fCrewedSrc | fExiledWith | fNotAttachedBy | fNamedCard
	for _, c := range f.Clauses {
		if c.Flags&srcFlags != 0 {
			return true
		}
	}
	return false
}

// valueReadsSource reports whether v (or a filter it counts) reads the
// static's source object.
func valueReadsSource(v lValue) bool {
	if nonSourceRef(v.Ref) {
		return true
	}
	for _, f := range []*lFilter{v.F, v.F2, v.F3, v.F4} {
		if filterReadsSource(f) {
			return true
		}
	}
	for _, s := range v.Sub {
		if valueReadsSource(s) {
			return true
		}
	}
	return false
}

// condReadsSource reports whether c reads the source or a resolution object.
func condReadsSource(c lCond) bool {
	if nonSourceRef(c.Ref) || valueReadsSource(c.A) || valueReadsSource(c.B) || filterReadsSource(c.F) {
		return true
	}
	for _, s := range c.Sub {
		if condReadsSource(s) {
			return true
		}
	}
	return false
}
