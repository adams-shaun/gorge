package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A Continuous static printed with EffectZone$ Graveyard (the Mirage
// Incarnations: Anger "As long as CARDNAME is in your graveyard and you control
// a Mountain, creatures you control have haste") works while its source card
// sits in its owner's graveyard and not on the battlefield (CR 113.6, 604.2;
// gorge layers.go staticEffectsWalk: chars.StaticZoneAdmitsStatic over the
// graveyard walk of staticSourceZones). The kernel reads it through
// kop.StFunctionsInGraveyard on a StatContinuous: the card's static is not a
// battlefield static (it never joins statM) and dirac/static_gycont.go yields
// it from each seat's graveyard, controlled by the owner.
//
// Only the keyword-grant shape the Incarnations print is lowered: the one
// static of the face, affecting battlefield objects with a plain keyword (or
// landwalk) grant under an IsPresent$ gate. A graveyard static beside another
// static of the same face, a P/T, subtype, ability, protection or ward grant,
// or any other EffectZone$ stays refused.

// graveContinuous reports whether s is a graveyard-scoped Continuous static
// and, when it is, refuses every shape the kernel does not read. The returned
// copy of s has EffectZone$ removed.
func (lc *lowerCtx) graveContinuous(s cards.Static) (cards.Static, bool, error) {
	if s.Mode != "Continuous" {
		return s, false, nil
	}
	ez, ok := s.Params["EffectZone"]
	if !ok || strings.TrimSpace(ez) != "Graveyard" {
		return s, false, nil
	}
	if _, kw := s.Params["AddKeyword"]; !kw {
		return s, false, nil // a graveyard MayPlay grant and the rest keep their own routes
	}
	for _, k := range []string{"MayPlay", "AffectedZone"} {
		if _, has := s.Params[k]; has {
			return s, false, nil // the graveyard grants of cards (flashback, escape, may-play) keep their own routes
		}
	}
	if len(lc.face.Statics) != 1 {
		return s, true, refuse("EffectZone$ Graveyard static beside another static of the face")
	}
	for _, k := range []string{"AddPower", "AddToughness", "AddType", "AddAbility", "AddTrigger", "AddStaticAbility", "RemoveKeyword", "RemoveAllAbilities", "AffectedDefined", "Goad", "ExcludeZone"} {
		if _, has := s.Params[k]; has {
			return s, true, refuse("EffectZone$ Graveyard static reading %s (only a keyword grant is read)", k)
		}
	}
	ps := make(map[string]string, len(s.Params))
	for k, v := range s.Params {
		if k != "EffectZone" {
			ps[k] = v
		}
	}
	s.Params = ps
	return s, true, nil
}

// gyContStFlag is the build-side half of graveContinuousFlag: the lowered
// static's "FunctionsInGraveyard" flag as kop.StFunctionsInGraveyard (works
// from its owner's graveyard).
func gyContStFlag(flags []string) kop.StaticFlags {
	if slices.Contains(flags, "FunctionsInGraveyard") {
		return kop.StFunctionsInGraveyard
	}
	return 0
}

// graveContinuousFlag marks the lowered statics as graveyard-functioning
// (kop.StFunctionsInGraveyard): exactly one plain Continuous keyword or
// landwalk grant, nothing the kernel's graveyard walk does not read.
func graveContinuousFlag(sts []lStatic) ([]lStatic, error) {
	n := 0
	for i := range sts {
		st := &sts[i]
		if st.Kind == "" {
			continue
		}
		n++
		if st.Kind != "Continuous" || st.F == nil || st.F.Players != 0 || st.Power.Kind != vNone || st.Tough.Kind != vNone ||
			len(st.Grant) != 0 || len(st.AddSub) != 0 || st.AddChosen || st.OutlivesLoss || st.Override != nil || st.ProtFrom != nil ||
			st.Ward != 0 || st.WardLife != 0 || len(st.Kw) == 0 && st.Landwalk == nil {
			return nil, refuse("EffectZone$ Graveyard static is not a plain keyword grant")
		}
		st.Flags = append(st.Flags, "FunctionsInGraveyard")
	}
	if n != 1 {
		return nil, refuse("EffectZone$ Graveyard static lowers to %d statics", n)
	}
	return sts, nil
}
