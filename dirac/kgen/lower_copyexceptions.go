package kgen

import (
	"slices"
	"strconv"
	"strings"
)

// the copy exceptions read once for both CopyPermanent
// subjects (one object, or Defined$ Valid <filter>).

// copyExceptions reads a CopyPermanent's copy exceptions (CR 707.9b) as one
// exception override, the copy's layer-1 copiable values (gorge
// CopyExcept); has is false when the ability names none.
func (lc *lowerCtx) copyExceptions(ps *params) (*lOverride, bool, error) {
	exc, has, _, err := lc.copyExceptionsMyriad(ps, false)
	return exc, has, err
}

// copyExceptionsMyriad is copyExceptions that, with allowMyriad, takes the
// bare "Myriad" terms of AddKeywords$ out of the exception and returns how
// many there were (the caller grants each as a latent trigger; an as-enters
// copy's grant record, cloneEnter). Without it a Myriad term refuses: a
// kernel copy exception carries keyword bits only. It never writes ps.
func (lc *lowerCtx) copyExceptionsMyriad(ps *params, allowMyriad bool) (*lOverride, bool, int, error) {
	var exc lOverride
	myriads := 0
	hasExc := false
	sp, okP := ps.get("SetPower")
	st, okT := ps.get("SetToughness")
	if okP || okT {
		p, e1 := strconv.Atoi(sp)
		t, e2 := strconv.Atoi(st)
		// a toughness below 1 would need the SBA shortcut to read the
		// exception (turn.go): refused
		if !okP || !okT || e1 != nil || e2 != nil || p < 0 || p > 127 || t < 1 || t > 127 {
			return nil, false, 0, refuse("CopyPermanent SetPower %q SetToughness %q", sp, st)
		}
		exc.HasSetPT, exc.SetP, exc.SetT, hasExc = true, p, t, true
	}
	if c, ok := ps.get("Controller"); ok && c != "You" {
		return nil, false, 0, refuse("CopyPermanent Controller %q", c)
	}
	if k, ok := ps.get("AddKeywords"); ok {
		// "except it has changeling" (CR 707.9b, 702.73a): gorge folds the
		// keyword into the token's basis face, where it is the printed
		// changeling's every-creature-type CDA; an all-creature-types copy
		// exception, seeded where the printed one is (copy_exceptions.go)
		var rest []string
		for _, w := range strings.Split(k, "&") {
			if w = strings.TrimSpace(w); w == "Changeling" {
				exc.AllCreatureTypes, hasExc = true, true
			} else {
				rest = append(rest, w)
			}
		}
		if slices.Contains(rest, "Myriad") {
			// gorge expands an added triggered keyword into the copy face's
			// own trigger (events foldCopyExcept, ApplyKeywordExpansion):
			// Auton Soldier, Muddle, the Ever-Changing (lower_myriad.go)
			if !allowMyriad {
				return nil, false, 0, refuse("copy exception AddKeywords$ Myriad: gorge gives the copy its own Attacks trigger; a kernel copy exception carries keyword bits only")
			}
			var left string
			left, myriads = splitMyriadGrant(strings.Join(rest, " & "))
			rest = nil
			if left != "" {
				rest = strings.Split(left, " & ")
			}
		}
		if len(rest) > 0 {
			ks, err := pumpKeywords(strings.Join(rest, " & "))
			if err != nil {
				return nil, false, 0, err
			}
			for _, w := range ks {
				if _, ok := kwBits[w]; !ok {
					return nil, false, 0, refuse("CopyPermanent AddKeywords %q", k)
				}
			}
			exc.AddKw, hasExc = ks, true
		}
	}
	if t, ok := ps.get("AddTypes"); ok {
		// gorge copyTypeList: split on "," and " & "; each word is a card
		// type, a supertype or a subtype, added and never replacing
		for _, part := range strings.Split(t, ",") {
			for _, w := range strings.Split(part, "&") {
				w = strings.TrimSpace(w)
				if b, ok := typeAddBit(w); ok {
					exc.AddTypes |= b
					continue
				}
				switch w {
				case "Plains", "Island", "Swamp", "Mountain", "Forest":
					return nil, false, 0, refuse("CopyPermanent AddTypes %q (a basic land type)", w)
				}
				if w == "" || !lc.known(w) {
					return nil, false, 0, refuse("CopyPermanent AddTypes %q", t)
				}
				exc.AddSubtypes = append(exc.AddSubtypes, w)
			}
		}
		hasExc = true
	}
	if c, ok := ps.get("AddColors"); ok {
		// additive to the copy's colours (gorge :200-210); "Colorless" adds
		// nothing and "All" is unmodeled here, so both refuse
		var letters strings.Builder
		for _, part := range strings.Split(c, ",") {
			for _, w := range strings.Split(part, "&") {
				switch strings.ToLower(strings.TrimSpace(w)) {
				case "white", "blue", "black", "red", "green":
				default:
					return nil, false, 0, refuse("CopyPermanent AddColors %q", c)
				}
				l, _ := colorWordSet(w)
				letters.WriteString(l)
			}
		}
		exc.AddColors, hasExc = letters.String(), true
	}
	if c, ok := ps.get("SetColor"); ok {
		// a colour exception (CR 707.9b; embalm's "white"): layer 1
		cols, ok := colorWordSet(c)
		if !ok {
			return nil, false, 0, refuse("CopyPermanent SetColor %q", c)
		}
		exc.HasSetColors, exc.SetColors, hasExc = true, cols, true
	}
	if v, ok := ps.get("AddStaticAbilities"); ok {
		// "except it has '<this Equipment's equip abilities cost {N} less>'"
		// (CR 707.9b; lower_copygrants.go)
		bit, err := lc.copyEquipGrant(v)
		if err != nil {
			return nil, false, 0, err
		}
		exc.GrantStatics, hasExc = bit, true
	}
	if ps.is("RemoveCost", "True") {
		// "with no mana cost" (CR 707.9b; embalm / eternalize): mana value 0
		exc.NoCost, hasExc = true, true
	}
	if v, ok := ps.get("NonLegendary"); ok {
		// the copy's type line loses Legendary (CR 707.9b; gorge
		// copypermanent.go removeLegendary), a copiable value
		if v != "True" {
			return nil, false, 0, refuse("CopyPermanent NonLegendary %q", v)
		}
		exc.RemoveLegendary, hasExc = true, true
	}
	return &exc, hasExc, myriads, nil
}
