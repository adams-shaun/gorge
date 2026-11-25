package kgen

import (
	"reflect"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A Continuous static whose AffectedZone$ is off the battlefield. Gorge
// applies a characteristic static to cards in hand, library and graveyard
// too (layers.go staticEffectsWalk: chars.StaticZoneAdmitsStatic), so the
// generic "AffectedZone %q without MayPlay" refusal would keep real carriers
// unlowered.
//
// The first slice lowers a type or keyword grant on exactly one of Hand,
// Graveyard, Library and All:
//   - a subtype grant (AddType$ ChosenType or a known subtype) rides
//     kop.StatOffZoneGrant with Static.Zone set, and the kernel projection
//     reads it for the card in that zone (dirac/featview.go
//     featViewTypesOff);
//   - a simple keyword with a KwMask bit rides the same static (Static.Kw),
//     projected through FeatGrantedOffRep;
//   - Retrace in a graveyard rides kop.StatRetraceGrant, and Unearth in a
//     graveyard rides kop.StatUnearthGrant (offZoneUnearth, kw_unearthgrant.go);
//     Cycling and TypeCycling in a hand ride kop.StatCyclingGrant
//     (offZoneCycling, kw_cyclinggrant.go);
//   - a hand Miracle grant rides kop.StatMiracleGrant (handMiracleGrant,
//     lower_miraclegrant.go; kw_miracle.go miracleCostOf).
//
// None of them is a kop.StatContinuous: the kernel's battlefield readers
// match a filter without reading its zone, so a Continuous static with an
// off-battlefield Zone would also grant the matching battlefield permanents.
//
// A card-type change, a colour change, a P/T change or an ability grant in
// those zones is a layer the kernel projects only on the battlefield; those
// shapes keep the generic refusal (the first slice's boundary). The keyword
// grants whose cast or activation offer is not modelled (Encore, Scavenge,
// Embalm, Dredge, Mayhem, Sneak, Foretell, Ninjutsu, Plot, ...) also keep it,
// Graveyard with its named reason. Measured over the corpus carriers: gorge
// mints no offer for Scavenge, Encore, Dredge, Mayhem, Sneak, Foretell or
// Plot from a Continuous grant (engine/cards/kw_granted.go), so their named
// refusal is lockstep-correct; Embalm's only carrier (Naktamun) and Cycling's
// Bicycle Rack are command-zone planes (EffectZone$ Command); Satoru
// Umezawa's Ninjutsu grant also needs an AbilityCast trigger.

// offZone maps an AffectedZone$ value to the kernel zone the static applies
// in. ZoneAny is All.
func offZone(az string) (kop.Zone, bool) {
	switch az {
	case "Hand":
		return kop.ZoneHand, true
	case "Graveyard":
		return kop.ZoneGraveyard, true
	case "Library":
		return kop.ZoneLibrary, true
	case "All":
		return kop.ZoneAny, true
	}
	return 0, false
}

// offZoneShape reports whether the static is one of the off-zone type or
// keyword grants this lowering reads. All's P/T grant (offZonePTShape) is
// deliberately not one: it keeps its own marker.
func offZoneShape(sp *params) bool {
	if _, ok := sp.get("AddKeyword"); ok {
		return true
	}
	if _, ok := sp.get("AddType"); ok {
		return true
	}
	return false
}

// offZoneEligible reports whether az is an off-zone single-zone shape this
// lowering handles (not a MayPlay grant).
func offZoneEligible(sp *params, az string) bool {
	_, ok := offZone(az)
	return ok && !sp.is("MayPlay", "True") && offZoneShape(sp)
}

// offZoneStatic lowers an off-zone type or keyword grant. handled is true
// when it consumed the whole static (the keyword arm); the type arm leaves
// the static to continuous' common tail, which parses the filter and the
// AddType$ words.
func (lc *lowerCtx) offZoneStatic(sp *params, st *lStatic, aff, az string, z kop.Zone) (handled bool, err error) {
	// A player-spec Affected$ filter would stamp a player half on the
	// off-zone grant, which gorge projects onto every card of the zone.
	if aff != "" {
		if f, ferr := lc.filter(aff); ferr == nil && f != nil && f.Players != 0 {
			return false, lc.refuseOffZoneKeyword(sp, az, z)
		}
	}
	if _, ok := sp.get("AddKeyword"); ok {
		return true, lc.offZoneKeyword(sp, st, aff, az, z)
	}
	return false, lc.offZoneType(sp, st, az, z)
}

// offZoneType validates the type-grant shape and marks the static's zone.
func (lc *lowerCtx) offZoneType(sp *params, st *lStatic, az string, z kop.Zone) error {
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "AddType", "Description", "Condition", "Secondary":
		default:
			return refuse("AffectedZone %q without MayPlay", az)
		}
	}
	t, ok := sp.get("AddType")
	if !ok {
		return refuse("AffectedZone %q without MayPlay", az)
	}
	for _, w := range strings.Split(t, "&") {
		w = strings.TrimSpace(w)
		if w == "ChosenType" || w == "AllBasicLandType" {
			continue
		}
		if !lc.known(w) {
			return refuse("AddType %q", w)
		}
		if _, cardType := typeAddBit(w); cardType {
			// a layer-4 card-type change is projected only on the
			// battlefield (the first slice's boundary)
			return refuse("AffectedZone %q without MayPlay", az)
		}
	}
	st.Zone = int(z)
	st.Flags = append(st.Flags, offZoneFlag)
	sp.consume("AffectedZone")
	return nil
}

// offZoneKeyword lowers a keyword grant in an off-battlefield zone: a
// simple KwMask keyword and Retrace in a graveyard. Every other keyword (or a grant beside another effect part) keeps its refusal.
func (lc *lowerCtx) offZoneKeyword(sp *params, st *lStatic, aff, az string, z kop.Zone) error {
	kw := strings.TrimSpace(sp.str("AddKeyword"))
	if kw == "" {
		return refuse("AffectedZone %q without MayPlay", az)
	}
	head, _, _ := strings.Cut(kw, ":")
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "AddKeyword", "Description", "Condition", "Secondary":
		case "EffectZone":
			// the static's source zone. Only the Unearth and the hand Cycling
			// grants read one: the battlefield source every battlefield
			// static walk sees (Sedris, the Traitor King; Rhet-Tomb Mystic).
			// Anything else keeps the refusal it holds today (Grixis's and
			// Bicycle Rack's command-zone source; every other keyword's
			// carrier).
			if head != "Unearth" && head != "Cycling" && head != "TypeCycling" || strings.TrimSpace(sp.str("EffectZone")) != "Battlefield" {
				return lc.refuseOffZoneKeyword(sp, az, z)
			}
		default:
			return lc.refuseOffZoneKeyword(sp, az, z)
		}
	}
	parts := strings.Split(kw, "&")
	simple := true
	for _, p := range parts {
		head, param, _ := strings.Cut(strings.TrimSpace(p), ":")
		if !simpleKeywords[head] || param != "" {
			simple = false
			break
		}
	}
	if simple {
		st.Zone = int(z)
		st.Flags = append(st.Flags, offZoneFlag)
		st.Kw = nil
		for _, p := range parts {
			st.Kw = append(st.Kw, strings.TrimSpace(p))
		}
		sp.consume("AddKeyword", "AffectedZone")
		return lc.offZoneFilter(sp, st, aff)
	}
	if len(parts) != 1 {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	head, param, _ := strings.Cut(strings.TrimSpace(parts[0]), ":")
	switch {
	case head == "Retrace" && param == "" && z == kop.ZoneGraveyard:
		c, err := lc.costHand("Discard<1/Land>")
		if err != nil || c.DiscardF == nil || c.DiscardN != 1 || c.DiscardSelf ||
			c.Mana.Generic != 0 || c.Mana.Pips != [5]int{} || c.Mana.C != 0 || c.Mana.X || c.Mana.XX ||
			len(c.Mana.Hybrid) != 0 || len(c.Mana.Twobrid) != 0 || len(c.Mana.Phyrexian) != 0 || c.Mana.XColor != 0 {
			return lc.refuseOffZoneKeyword(sp, az, z)
		}
		st.Kind = "RetraceGrant"
		st.Zone = int(z)
		st.Kw = []string{"Retrace"} // the LIR dump's name for it; the build reads the kind
		st.Cost = c
	case head == "Unearth" && z == kop.ZoneGraveyard:
		return lc.offZoneUnearth(sp, st, aff, az, z, param)
	case (head == "Cycling" || head == "TypeCycling") && z == kop.ZoneHand:
		return lc.offZoneCycling(sp, st, aff, az, z, head, param)
	default:
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	sp.consume("AddKeyword", "AffectedZone")
	return lc.offZoneFilter(sp, st, aff)
}

// refuseOffZoneKeyword keeps the census's existing reason for an unsupported
// off-zone grant: a Graveyard keyword grant names its keyword
// (refuseGraveKwGrant), every other one the generic AffectedZone reason.
func (lc *lowerCtx) refuseOffZoneKeyword(sp *params, az string, z kop.Zone) error {
	if z == kop.ZoneGraveyard && sp.str("AddKeyword") != "" {
		return refuseGraveKwGrant(sp, az)
	}
	return refuse("AffectedZone %q without MayPlay", az)
}

// offZoneUnearth lowers an AffectedZone$ Graveyard Unearth grant (CR
// 702.84a). gorge's derived-keyword walk mints the printed kwUnearth
// expander's one AB$ ChangeZone (Origin$ Graveyard, Destination$ Battlefield,
// Unearth$ True) on the affected card through cards.GrantedKeywordAbility
// (legal_granted.go's granted-keyword block) and offers it from the
// graveyard, so the grant lowers as one kop.StatUnearthGrant static naming
// the same body as the granting card's latent ability (grantedKeywordLatent):
// the kernel's graveyard offer (dirac kw_unearthgrant.go) runs it with the
// affected card as its source, and the OpUnearth the body carries applies the
// haste and the exile-at-end-step riders the printed activation applies.
//
// The cost is the grant's own. An explicit mana cost ("Unearth:2 B") is baked
// into the body, so the offer reads it off the body; a CardManaCost grant
// (Grixis prices each card's own mana cost) bakes the placeholder zero and is
// priced by the affected card's printed mana cost -- a zero body cost is the
// CardManaCost mark (a plain explicit zero refuses: no corpus carrier prices
// unearth at nothing, and it would be indistinguishable). A grant whose cost
// is not plain mana (an {X}, a hybrid pip, an XAlternative$ like Graywater's
// Fixer's encore) or whose body the expander cannot mint refuses by name.
func (lc *lowerCtx) offZoneUnearth(sp *params, st *lStatic, aff, az string, z kop.Zone, param string) error {
	// one grant per card: the reader matches the first static, so a second
	// Unearth static (a different cost on the same card) would price wrong.
	// No corpus card carries two.
	n := 0
	for _, fs := range lc.face.Statics {
		if fs.Params["AffectedZone"] == "Graveyard" && strings.HasPrefix(strings.TrimSpace(fs.Params["AddKeyword"]), "Unearth") {
			n++
		}
	}
	if n > 1 {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	param = strings.TrimSpace(param)
	if param == "" || param == "0" {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	line := "Unearth:" + param
	if param == "CardManaCost" {
		// the affected card's own mana cost: the body bakes the placeholder
		// zero, the offer and the charge read the card's printed cost
		line = "Unearth:0"
	} else {
		c, err := lc.cost(param)
		if err != nil || !plainGrantMana(c) {
			return lc.refuseOffZoneKeyword(sp, az, z)
		}
	}
	sa := cards.GrantedKeywordAbility(line)
	if sa == nil || sa.Kind != "AB" {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	idx, err := lc.grantedKeywordLatent(sa)
	if err != nil {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	st.Kind = "UnearthGrant"
	st.Zone = int(z)
	st.GrantBody = idx
	sp.consume("AddKeyword", "AffectedZone", "EffectZone")
	return lc.offZoneFilter(sp, st, aff)
}

// plainGrantMana reports whether the cost c is a plain mana cost an off-zone
// keyword grant's offer can price: no discard part, no {X}, no hybrid,
// Phyrexian or snow pip, no explicit no-cost mark.
func plainGrantMana(c lCost) bool {
	return c.DiscardF == nil && c.DiscardN == 0 && !c.DiscardHand && !c.DiscardSelf &&
		!c.Mana.X && !c.Mana.XX && !c.Mana.XXX && !c.Mana.None && c.Mana.XColor == 0 && c.Mana.Snow == 0 &&
		len(c.Mana.Hybrid) == 0 && len(c.Mana.Twobrid) == 0 && len(c.Mana.Phyrexian) == 0
}

// offZoneCycling lowers an AffectedZone$ Hand Cycling or TypeCycling grant
// (CR 702.29a, 702.29e). gorge's derived-keyword walk mints the printed
// cycling body (cards.GrantedKeywordAbility: AB$ Draw or the typecycling
// search, Cost$ <cost> Discard<1/CARDNAME>, ActivationZone$ Hand) on the
// affected hand card and offers it from the hand, so the grant lowers as one
// kop.StatCyclingGrant static naming the same body as the granting card's
// latent ability (grantedKeywordLatent): the kernel's hand offer (dirac
// kw_cyclinggrant.go) runs it with the affected card as its source, which the
// body's own discard-this cost moves to the graveyard.
//
// The cost is the grant's own and must be plain mana (the body bakes it). A
// second cycling grant on one card (no corpus carrier has one) keeps the
// refusal: the reader matches the first static. A grant whose cost is not
// plain mana, whose body the expander cannot mint or whose static is read
// from off the battlefield (Bicycle Rack's command-zone plane) refuses by
// name.
func (lc *lowerCtx) offZoneCycling(sp *params, st *lStatic, aff, az string, z kop.Zone, head, param string) error {
	n := 0
	for _, fs := range lc.face.Statics {
		h := strings.TrimSpace(fs.Params["AddKeyword"])
		if fs.Params["AffectedZone"] == "Hand" && (strings.HasPrefix(h, "Cycling") || strings.HasPrefix(h, "TypeCycling")) {
			n++
		}
	}
	if n > 1 {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	param = strings.TrimSpace(param)
	cost := param
	if head == "TypeCycling" {
		// "TypeCycling:<type>:<cost>"
		_, cost, _ = strings.Cut(param, ":")
		cost = strings.TrimSpace(cost)
	}
	if cost == "" {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	c, err := lc.cost(cost)
	if err != nil || !plainGrantMana(c) || !reflect.DeepEqual(c, lCost{Mana: c.Mana}) {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	sa := cards.GrantedKeywordAbility(head + ":" + param)
	if sa == nil || sa.Kind != "AB" {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	idx, err := lc.grantedKeywordLatent(sa)
	if err != nil {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	if b := lc.out.Abilities[idx]; !b.Cycling || !b.Cost.DiscardSelf {
		return lc.refuseOffZoneKeyword(sp, az, z)
	}
	st.Kind = "CyclingGrant"
	st.Zone = int(z)
	st.GrantBody = idx
	sp.consume("AddKeyword", "AffectedZone", "EffectZone")
	return lc.offZoneFilter(sp, st, aff)
}

// offZoneFilter parses the Affected$ filter of an off-zone keyword grant.
func (lc *lowerCtx) offZoneFilter(sp *params, st *lStatic, aff string) error {
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	st.F = f
	return nil
}

// offZoneFlag marks a lowered Continuous static as an off-zone type or
// keyword grant: the build makes it a kop.StatOffZoneGrant, never a
// StatContinuous.
const offZoneFlag = "OffZoneGrant"

// allZoneTwin splits an AffectedZone$ All grant into the two statics gorge
// applies it as: the plain battlefield Continuous (the battlefield readers
// never see a StatOffZoneGrant) and the off-zone grant for every other zone.
func (sc *staticCtx) allZoneTwin() {
	st := &sc.st
	if sc.extra != nil || st.Zone != int(kop.ZoneAny) || !slices.Contains(st.Flags, offZoneFlag) {
		return
	}
	bf := *st
	bf.Zone = 0
	bf.Flags = slices.DeleteFunc(slices.Clone(st.Flags), func(f string) bool { return f == offZoneFlag })
	sc.extra = []lStatic{bf, *st}
}

// continuousKind sets the kind of a built Continuous static and returns the
// zone its filter is read in: the battlefield, or for an off-zone grant
// (offZoneFlag) the static's own AffectedZone$ (ZoneAny, from All, is a
// zone-free filter, so it matches the card in any zone).
func continuousKind(st *kop.Static, flags []string, z int) kop.Zone {
	if !slices.Contains(flags, offZoneFlag) {
		st.Kind = kop.StatContinuous
		return kop.ZoneBattlefield
	}
	st.Kind = kop.StatOffZoneGrant
	st.Zone = kop.Zone(z)
	if st.Zone == kop.ZoneAny {
		return kop.ZoneNone
	}
	return st.Zone
}

// offKwGrantAbility builds a StatRetraceGrant static: Static.Zone and the
// filter name the affected zone, and the ability's own Cost is the retrace
// additional cost (Discard<1/Land>).
func (cb *cardBuilder) offKwGrantAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatRetraceGrant // kw_offkwgrant.go
	st.Zone = kop.Zone(s.Zone)
	z := st.Zone
	if z == kop.ZoneAny {
		z = kop.ZoneNone
	}
	if s.F != nil {
		f, err := cb.filter(s.F, z)
		if err != nil {
			return ka, nil, err
		}
		st.Filter = f
	}
	cost, err := cb.cost(s.Cost)
	if err != nil {
		return ka, nil, err
	}
	ka.Cost = cost
	return ka, nil, nil
}

// affectedZone is the AffectedZone$ dispatch of continuous (lower_continuous.go):
// the battlefield default, the named Stack/Graveyard/Hand grants, the
// off-zone type and keyword grants, and the generic refusal. done reports
// that the static was handled (lowered or refused) and continuous must
// return; the off-zone type arm leaves the static to continuous' common tail.
func (lc *lowerCtx) affectedZone(sp *params, st *lStatic, aff, az string) (extra []lStatic, done bool, err error) {
	switch {
	case az == "Battlefield":
		return nil, false, nil // the default
	case az == "Stack" && !sp.is("MayPlay", "True"):
		extra, err = lc.stackKwGrant(sp, st, aff) // lower_stackkw.go
		return extra, true, err
	case az == "Graveyard" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Flashback"):
		return nil, true, lc.graveFbGrant(sp, st, aff) // lower_fbgrant.go
	case az == "Graveyard" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Escape"):
		return nil, true, lc.graveEscapeGrant(sp, st, aff) // lower_escapegrant.go
	case az == "Hand" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Warp"):
		return nil, true, lc.handWarpGrant(sp, st, aff) // lower_warpgrant.go
	case az == "Hand" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Miracle"):
		return nil, true, lc.handMiracleGrant(sp, st, aff) // lower_miraclegrant.go
	case az == "Graveyard" && !sp.is("MayPlay", "True") && inertCountAs(sp.str("AddHiddenKeyword"), aff):
		return nil, true, graveCountAsStatic(sp, st) // lower_escapegrant.go
	case az == "All" && offZonePTShape(sp, aff):
		return nil, true, offZonePTStatic(sp, st) // lower_offzonept.go
	case offZoneEligible(sp, az):
		z, _ := offZone(az)
		handled, err := lc.offZoneStatic(sp, st, aff, az, z) // the type/keyword arms above
		return nil, handled, err
	case az == "Graveyard" && !sp.is("MayPlay", "True") && sp.str("AddKeyword") != "":
		return nil, true, refuseGraveKwGrant(sp, az) // lower_escapegrant.go
	case !sp.is("MayPlay", "True"):
		return nil, true, refuse("AffectedZone %q without MayPlay", az)
	}
	return nil, false, nil
}
