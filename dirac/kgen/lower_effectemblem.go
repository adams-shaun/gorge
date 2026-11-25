package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectStaticEOT lowers an Effect with neither ValidTgts$ nor
// RememberObjects$ whose StaticAbilities$ bodies are modes gorge's effEffect
// REGISTERS (effects/effect.go: the CantBlockBy, MustAttack and CantBeCast
// restriction cases, the AddHiddenKeyword$ / AdjustLandPlays$ /
// SetMaxHandSize$ special grants, and the general Mode$ Continuous layer
// grant) as emblems of the already-lowered statics. Lifetime follows the
// Effect's Duration$: absent / EndOfTurn / UntilEndOfTurn /
// UntilYourNextEndStep are the kernel's EOT emblem; UntilYourNextTurn is the
// next-turn emblem; every other spelling is refused because the kernel
// cannot model its boundary. The direct CantBlock / CantAttack modes stay
// refused until their own lowering reads them from an emblem.
func (lc *lowerCtx) effectStaticEOT(ps *params, ss []cards.Static) error {
	// an Effect whose only statics are ReduceCost
	// Count$ChosenNumber bodies freezes its SetChosenNumber$ at resolution
	// (Rowan, Scion of War; Will, Scion of Peace). Any other SetChosenNumber$
	// shape refuses by name rather than lowering the read as a zero.
	chosen, embNum, err := lc.effectEmblemChosenNumber(ps, ss)
	if err != nil {
		return err
	}
	activated := lc.ab != nil && lc.ab.Kind == akActivated
	// a triggered Effect with no Duration$ takes the
	// same host-bound allSource arm as an activated one (never the
	// UntilHostLeavesPlayOrEOT spelling, which stays activated-only).
	srcBound := activated || lc.ab != nil && lc.ab.Kind == akTriggered
	host := ps.is("Duration", "UntilHostLeavesPlayOrEOT") && activated // lower_tokenattacking.go
	if host {
		ps.consume("Duration")
	} else if _, hasDur := ps.m["Duration"]; !hasDur && srcBound {
		// An activated or triggered Effect with no Duration$ whose every
		// static reads the effect's source is the same host-bound emblem
		// (Krotiq Nestguard, Territorial Witchstalker, Stalked Researcher).
		// EffectSource is an
		// alias of the registered effect's source object (gorge
		// effects/filter.go), and a card that leaves and returns is a new
		// object (CR 400.7), so "until end of turn" and "until the source
		// leaves" are observably the same boundary: the kernel's WhileHost
		// emblem. Every body must be the CantBlockBy ValidAttacker or
		// CanAttackDefender ValidCard spelling; effectSourceSelf refuses any
		// other EffectSource use.
		allSource := len(ss) > 0
		for _, s := range ss {
			if _, found, err := effectSourceSelf(s); err != nil {
				if activated {
					return err
				}
				// a triggered Effect keeps the refusal its own shape named
				// before this arm (Raphael's ExiledWithEffectSource is not
				// an EffectSource filter)
				allSource = false
			} else if !found {
				allSource = false
			}
		}
		if allSource {
			host = true
			// ExileOnMoved$ Battlefield is the host-leaves end some carriers
			// spell explicitly (Verdant Outrider); Defined$ Self is the
			// Effect key gorge records but does not read (Giant Koi).
			if xm, ok := ps.get("ExileOnMoved"); ok {
				if xm != "Battlefield" {
					return refuse("Effect static ExileOnMoved$ %q", xm)
				}
			}
			if d, ok := ps.get("Defined"); ok {
				if d != "Self" {
					return refuse("Effect static Defined$ %q", d)
				}
			}
			// ForgetOnMoved$ only drops REMEMBERED cards (gorge effect.go);
			// with no RememberObjects$ it is inert (Witchstalker, Stalked
			// Researcher).
			if fm, ok := ps.get("ForgetOnMoved"); ok {
				if _, hasRO := ps.m["RememberObjects"]; hasRO {
					return refuse("Effect static ForgetOnMoved$ with RememberObjects$")
				}
				if fm != "Battlefield" {
					return refuse("Effect static ForgetOnMoved$ %q", fm)
				}
			}
			ps.consume("ExileOnMoved", "Defined", "ForgetOnMoved")
		}
	}
	eot, next, durHost, err := true, false, false, error(nil)
	if !host {
		if eot, next, durHost, err = effectEmblemDuration(ps); err != nil {
			return err
		}
	}
	host = host || durHost
	sts := make([]lStatic, 0, len(ss))
	for _, s := range ss {
		if host {
			if s, _, err = effectSourceSelf(s); err != nil {
				return err
			}
		}
		st, err := lc.effectEmblemStaticHost(s, host)
		if err != nil {
			return err
		}
		if st.Kind == "" {
			continue // the view-only face-down look lowers to no static
		}
		if embNum {
			// the amount is the emblem's frozen number, not a value
			st.Flags = append(st.Flags, "EmbNum")
		}
		sts = append(sts, st)
	}
	var forget *lFilter
	if fc, ok := ps.get("ForgetOnCast"); ok {
		// gorge effectCastSweep: the first completed cast the spec names ends
		// the effect. A spell-flash grant reads it as the CastWithFlash
		// static's Filter2 (Quicken's "the next sorcery spell you cast this
		// turn"; kernel embFlashForget). A stack keyword grant (Cascade,
		// Improvise, Convoke, Affinity) is read live at cast time, so the
		// emblem itself carries the filter and ends on the matching cast
		// (kop.Emblem.Forget; kernel embForgetOnCast).
		if fc != "False" {
			if len(sts) != 1 {
				return refuse("Effect ForgetOnCast$ beside %d statics", len(sts))
			}
			f, err := lc.filter(fc)
			if err != nil {
				return err
			}
			switch sts[0].Kind {
			case "CastWithFlash":
				sts[0].F2 = f
			case "StackKw":
				forget = f
			case "ReduceCost":
				// only an Affinity keyword grant (it carries the keyword): a plain
				// Mode$ ReduceCost beside ForgetOnCast$ (Invasion of the Giants) is
				// gorge's effectCastSweep over a cost reduction, which stays refused
				if len(sts[0].Kw) == 0 {
					return refuse("Effect ForgetOnCast$ beside a %s static", sts[0].Kind)
				}
				forget = f
			default:
				return refuse("Effect ForgetOnCast$ beside a %s static", sts[0].Kind)
			}
		}
		ps.consume("ForgetOnCast")
	}
	ps.consume("Name", "Image", "Stackable")
	left := ps.unread()
	// The Effect's own Condition*$ keys (ConditionPresent$, the monarch's
	// ConditionCheckSVar$) guard the whole node: the node's condition read
	// (nodeCondition, lower_effect.go) runs after this lowering, puts a
	// SkipIf over the node alone, and refuses any key it does not read.
	// gorge checks a sub-ability's conditions at resolution for every API
	// (engine/effects/conditions.go), so the deferral is not a narrowing.
	left = slices.DeleteFunc(left, func(k string) bool { return strings.HasPrefix(k, "Condition") })
	if len(left) > 0 {
		return refuse("Effect static unread %v", left)
	}
	if len(sts) == 0 {
		// gorge registers no continuous effect for the
		// view-only look, so the Effect resolves without creating anything.
		return nil
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: sts, UntilEOT: eot, UntilNextTurn: next, WhileHost: host, Forget: forget})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1, A: chosen})
	return nil
}

// effectEmblemChosenNumber reads a no-target Effect's SetChosenNumber$ for
// the one emblem shape the kernel freezes: every
// StaticAbilities$ body is a Mode$ ReduceCost Type$ Spell static whose
// Amount$ names an SVar whose whole body is Count$ChosenNumber. It replaces
// each body's Amount$ with the constant 0 (StEmbNum: the static lowers with
// no dynamic value and no VA) and consumes SetChosenNumber$ and returns its
// lowered value for the CreateEmblem instruction's A. Anything else refuses
// by name: a SetChosenNumber$ outside this shape must not lower as a silent
// zero.
func (lc *lowerCtx) effectEmblemChosenNumber(ps *params, ss []cards.Static) (lValue, bool, error) {
	name, ok := ps.m["SetChosenNumber"]
	if !ok {
		return lValue{}, false, nil
	}
	if _, ok := ps.m["Triggers"]; ok || len(ss) == 0 {
		return lValue{}, false, refuse("Effect SetChosenNumber$ outside an emblem ReduceCost")
	}
	if _, ok := ps.m["ReplacementEffects"]; ok {
		return lValue{}, false, refuse("Effect SetChosenNumber$ outside an emblem ReduceCost")
	}
	for i := range ss {
		s := &ss[i]
		if s.Mode != "ReduceCost" || s.Params["Type"] != "Spell" {
			return lValue{}, false, refuse("Effect SetChosenNumber$ outside an emblem ReduceCost")
		}
		body, ok := lc.face.SVars[s.Params["Amount"]]
		if !ok || strings.TrimSpace(body) != "Count$ChosenNumber" {
			return lValue{}, false, refuse("Effect SetChosenNumber$ outside an emblem ReduceCost")
		}
		cp := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			cp[k] = v
		}
		cp["Amount"] = "0"
		s.Params = cp
	}
	v, err := lc.value(strings.TrimSpace(name))
	if err != nil {
		return lValue{}, false, err
	}
	ps.consume("SetChosenNumber")
	return v, true, nil
}

// effectEmblemDuration maps an Effect line's Duration$ to the emblem
// lifetimes the kernel models. effectUntilEOT accepts these spellings as the
// this-turn boundary; IsNextTurnDuration makes UntilYourNextTurn a real
// start-of-next-turn boundary. UntilTheEndOfYourNextTurn is also a
// next-turn duration in gorge, but it lasts through that turn's cleanup, a
// lifetime the kernel's emblem record does not carry, so it refuses. So does
// Duration$ Permanent here: only effectEmblemDurationAs with allowPermanent
// reads it (effectEmblemPrevent, lower_effectemblemprevent.go).
func effectEmblemDuration(ps *params) (eot, next, host bool, err error) {
	return effectEmblemDurationAs(ps, false)
}

// effectEmblemDurationAs is effectEmblemDuration; allowPermanent reads
// Duration$ Permanent as the emblem with none of the three flags, which no
// cleanup drops.
func effectEmblemDurationAs(ps *params, allowPermanent bool) (eot, next, host bool, err error) {
	d, ok := ps.get("Duration")
	if !ok {
		return true, false, false, nil
	}
	switch d {
	case "EOT", "EndOfTurn", "UntilEndOfTurn", "UntilYourNextEndStep", "ThisTurn":
		return true, false, false, nil
	case "UntilYourNextTurn":
		return false, true, false, nil
	case "Permanent":
		if allowPermanent {
			return false, false, false, nil
		}
		return false, false, false, refuse("Effect static Duration$ %q", d)
	case "UntilHostLeavesPlayOrEOT":
		// "until end of turn, or until the host leaves the battlefield,
		// whichever is first": both end conditions, joined by OR. The
		// kernel's WhileHost emblem ends at the host's leave and its
		// UntilEOT flag at cleanup, so the record carries both.
		return true, false, true, nil
	default:
		return false, false, false, refuse("Effect static Duration$ %q", d)
	}
}

// effectEmblemStaticHost is effectEmblemStatic; host: the emblem reads its
// host as the source (Self filters allowed).
func (lc *lowerCtx) effectEmblemStaticHost(s cards.Static, host bool) (lStatic, error) {
	var keys map[string]bool
	checkKeys := true
	libTop, libPayLife := false, false // the library-top look-and-play grant (lower_effectlibtop.go)
	mustAttackOther := false
	switch s.Mode {
	case "CantBlockBy":
		keys = map[string]bool{"Mode": true, "ValidAttacker": true, "ValidBlocker": true, "Description": true, "Secondary": true}
	case "MustAttack":
		keys = map[string]bool{"Mode": true, "ValidCreature": true, "MustAttack": true, "Description": true, "Secondary": true}
		if v := s.Params["MustAttack"]; v != "" && v != "Player.Other" {
			return lStatic{}, refuse("Effect static MustAttack$ %q", v)
		}
		if s.Params["MustAttack"] != "" {
			// MustAttack$ Player.Other is seat-exact: the requirement stays
			// and the static carries kop.StatMustAttack's B mark, so the kernel
			// withholds the static's controller as a defender while another
			// living player is a legal one (combat.go attackOffersN). At two
			// seats that is the one opponent, as before.
			mustAttackOther = true
		}
		// a bare MustAttack (Bident of Thassa) is "attacks if able" with any
		// defender (CR 508.1d), seat-exact: at more than two seats the kernel
		// asks each such attacker's defender as gorge does (mustAttackAsked).
		// staticBody has no field for the player reference; the flag above
		// carries the defender restriction, so drop the key before
		// lowering.
		cp := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			cp[k] = v
		}
		delete(cp, "MustAttack")
		s.Params = cp
	case "Continuous":
		checkKeys = false
		if rest, pay, ok := effectLibTopPlay(s); ok {
			s, libTop, libPayLife = rest, true, pay
			break
		}
		// a body that is only the view-only face-down
		// battlefield look lowers to no static: gorge's effEffect registers a
		// look-only grant, an information read the kernel has no static for.
		// effectStaticEOT drops the empty static and, when the Effect has no
		// other static, emits no CreateEmblem at all.
		if faceDownLookOnly(s.Params) {
			return lStatic{Kind: ""}, nil
		}
		if err := effectContinuousKeys(s.Params); err != nil {
			return lStatic{}, err
		}
		if s.Params["Affected"] == "You" {
			if _, ok := s.Params["SetMaxHandSize"]; !ok {
				if _, ok := s.Params["AdjustLandPlays"]; !ok {
					return lStatic{}, refuse("Effect static player Continuous without SetMaxHandSize/AdjustLandPlays")
				}
			}
		}
	case "CantBeCast":
		// gorge cantBeCastParamsReadableKeys; the lowering consumes the
		// subset the kernel's cast gate reads, the rest refuses as unread
		keys = map[string]bool{"Mode": true, "ValidCard": true, "Caster": true, "Origin": true, "NumLimitEachTurn": true, "AffectedZone": true, "Phases": true, "Condition": true, "ClassBand": true, "IsPresent": true, "PresentCompare": true, "PresentZone": true, "CheckSVar": true, "SVarCompare": true, "Description": true}
	case "CastWithFlash":
		if lc.face != nil && lc.face.Name == "Complete the Circuit" {
			// the grant beside a delayed SpellCast copy trigger: the lockstep
			// (effectflash_test.go) offers its {5}{U} convoke cast too rarely
			// for the pair to play identically with evidence
			return lStatic{}, refuse("Effect static CastWithFlash beside a delayed SpellCast copy trigger (Complete the Circuit): its {5}{U} convoke cast is not reached by any lockstep game")
		}
		// gorge effects.SpellFlashParamsReadable: Caster$ You (the effect's
		// controller, the emblem's player) and ValidSA$ Spell. AffectedZone$
		// (Winding Canyons' every-zone list) is read by neither side, so it
		// is dropped like MustAttack's player reference above.
		keys = map[string]bool{"Mode": true, "ValidCard": true, "ValidSA": true, "Caster": true, "AffectedZone": true, "Description": true, "Secondary": true}
		cp := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			cp[k] = v
		}
		delete(cp, "AffectedZone")
		s.Params = cp
	case "CantPreventDamage":
		keys = map[string]bool{"Mode": true, "Description": true, "Secondary": true} // the global form (gorge cantPreventDamage: no Affected$)
	case "CanAttackDefender":
		// gorge canAttackDefenderGrantParamsReadableKeys
		keys = map[string]bool{"Mode": true, "ValidCard": true, "ValidCards": true, "ValidTarget": true, "ValidAttacked": true, "Description": true, "Secondary": true}
	case "AssignNoCombatDamage":
		// gorge effectCombatStatic's AssignNoCombatDamage restriction
		// (effect_combatstatics.go), read by rules' combatDamageAmount; the
		// kernel zeroes the matching creature's assignment in the combat
		// damage step. ValidCard$ names the effect's source (rewritten to
		// Self by effectSourceSelf, the host the emblem reads as its source).
		keys = map[string]bool{"Mode": true, "ValidCard": true, "Description": true}
	case "ReduceCost":
		// gorge costStaticParamsReadableKeys
		keys = map[string]bool{"Mode": true, "Type": true, "ValidCard": true, "ValidSA": true, "ValidPlayer": true, "ValidSpell": true, "ValidTarget": true, "Activator": true, "Caster": true, "Amount": true, "Cost": true, "Announce": true, "Color": true, "MinMana": true, "IgnoreGeneric": true, "RaiseTo": true, "OnlyFirstSpell": true, "IsPresent": true, "PresentZone": true, "PresentCompare": true, "CheckSVar": true, "SVarCompare": true, "CheckSecondSVar": true, "SecondSVarCompare": true, "Condition": true, "EffectZone": true, "AffectedZone": true, "Secondary": true, "Relative": true, "ClassBand": true, "UnlessValidTarget": true, "PlayerTurn": true, "Phases": true, "Description": true, "SpellDescription": true}
	default:
		return lStatic{}, refuse("Effect static mode %s without RememberObjects$", s.Mode)
	}
	if checkKeys {
		for k := range s.Params {
			if !keys[k] {
				return lStatic{}, refuse("Effect static %s param %s without RememberObjects$", s.Mode, k)
			}
		}
	}
	sts, err := lc.staticBody(s)
	if err != nil {
		return lStatic{}, err
	}
	if len(sts) != 1 {
		return lStatic{}, refuse("Effect static %s lowers to %d statics", s.Mode, len(sts))
	}
	st := sts[0]
	if mustAttackOther {
		st.Flags = append(st.Flags, "MustAttackOther")
	}
	if libTop {
		if st.Kind != "MayPlayFrom" {
			return lStatic{}, refuse("Effect static library-top play lowered to %s", st.Kind)
		}
		if libPayLife {
			st.Flags = append(st.Flags, "MayPayLife")
		}
	} else if s.Mode == "CastWithFlash" {
		if st.Kind != "CastWithFlash" {
			return lStatic{}, refuse("Effect static CastWithFlash lowered to %s", st.Kind)
		}
	} else if s.Mode == "CantBeCast" {
		if st.Kind != "CantBeCast" || st.Zone == zAny {
			// Card.Self binds the card's own object; an emblem has none
			return lStatic{}, refuse("Effect static CantBeCast lowered to %s over its own card", st.Kind)
		}
	} else if s.Mode == "Continuous" {
		switch st.Kind {
		case "CantBlock", "ExtraLandPlay", "NoMaxHandSize", "Continuous", "Override",
			// an Effect-issued stack keyword grant: "the next spell you cast
			// this turn has cascade / improvise / convoke / affinity"
			"StackKw", "ReduceCost":
		default:
			return lStatic{}, refuse("Effect static Continuous lowered to %s", st.Kind)
		}
	}
	if st.Kind == "StackKw" || st.Kind == "ReduceCost" {
		// an EffectZone$ Stack self grant is a printed-static shape: the
		// kernel reads it off the spell's own card abilities, never an emblem
		for _, fl := range st.Flags {
			if fl == "SelfStack" {
				return lStatic{}, refuse("Effect static stack grant with EffectZone Stack")
			}
		}
		if st.Kind == "StackKw" && len(st.Kw) == 1 && st.Kw[0] == "Rebound" {
			// a granted Rebound runs the carrier card's recast body
			// (kw_rebound.go reboundPlan); an emblem has no card
			return lStatic{}, refuse("Effect static stack grant of Rebound")
		}
	}
	if host {
		if err := emblemFiltersRefuseHost("Effect static", st.F, st.F2); err != nil {
			return lStatic{}, err
		}
		return st, nil
	}
	if err := emblemFiltersRefuse("Effect static", st.F, st.F2); err != nil {
		return lStatic{}, err
	}
	return st, nil
}

// emblemFiltersRefuse refuses an emblem static or trigger filter that reads
// the effect's source or the choice it made. An emblem has no source
// permanent: its statics are evaluated with sl == NoSlot, obj == NoObj
// (dirac/chars_gen.go), so a ChosenType/ChosenColor filter would read the
// resolution's answer instead of the permanent's choice (Walking
// Desecration's MustAttack over Creature.ChosenType would never force an
// attack). where names the lowering in the refusal.
func emblemFiltersRefuse(where string, fs ...*lFilter) error {
	for _, f := range fs {
		if f == nil {
			continue
		}
		for _, c := range f.Clauses {
			if c.Flags&(fSelf|fOther) != 0 {
				return refuse("%s filter %s names the effect's source", where, f)
			}
			if c.Flags&(fChosenType|fChosenColor) != 0 {
				return refuse("%s filter %s reads the effect's chosen type or colour", where, f)
			}
		}
	}
	return nil
}

// effectContinuousKeys is the parameter whitelist of the three special
// Mode$ Continuous Effect branches plus the general layer grant in gorge's
// effEffect. The special branches have their own narrower key sets: an
// AdjustLandPlays$ body with AffectedZone$ (or an AddHiddenKeyword$ body with
// a second grant) must refuse, because the reference branch would not
// register it.
func effectContinuousKeys(p map[string]string) error {
	special := 0
	if _, ok := p["AddHiddenKeyword"]; ok {
		special++
		for _, k := range sortedParamKeys(p) {
			switch k {
			case "Mode", "Affected", "AffectedZone", "Description", "AddHiddenKeyword":
			default:
				return refuse("Effect static Continuous param %s beside AddHiddenKeyword$", k)
			}
		}
		if v := p["AddHiddenKeyword"]; v != "CARDNAME can't block." {
			return refuse("Effect static AddHiddenKeyword$ %q", v)
		}
	}
	if _, ok := p["AdjustLandPlays"]; ok {
		special++
		for _, k := range sortedParamKeys(p) {
			switch k {
			case "Mode", "Affected", "Description", "AdjustLandPlays":
			default:
				return refuse("Effect static Continuous param %s beside AdjustLandPlays$", k)
			}
		}
	}
	if _, ok := p["SetMaxHandSize"]; ok {
		special++
		for _, k := range sortedParamKeys(p) {
			switch k {
			case "Mode", "Affected", "AffectedZone", "Description", "SetMaxHandSize":
			default:
				return refuse("Effect static Continuous param %s beside SetMaxHandSize$", k)
			}
		}
	}
	if special > 1 {
		return refuse("Effect static Continuous with several special grants")
	}
	if special != 0 {
		return nil
	}
	for _, k := range sortedParamKeys(p) {
		switch k {
		case "Mode", "Affected", "AffectedZone", "Description",
			"AddTypes", "AddType", "AddAllCreatureTypes", "RemoveCreatureTypes",
			"RemoveCardTypes", "RemoveLandTypes", "AddColor", "AddColors",
			"SetColor", "SetColors", "AddKeyword", "AddAbility", "AddAbilities",
			"RemoveAllAbilities", "SetPower", "SetToughness", "AddPower", "AddToughness":
		default:
			return refuse("Effect static Continuous param %s", k)
		}
	}
	return nil
}

// effectSourceSelf rewrites a host-bound Effect static's EffectSource filter
// head to Self (gorge's EffectSource is the Effect's source card, the host
// the emblem reads as its source; kernel eachStatic). Only CantBlockBy's
// attacker side (Gingerbrute's Creature.EffectSource, Giant Koi's and Verdant
// Outrider's Card.EffectSource), CanAttackDefender's ValidCard$ (Krotiq
// Nestguard's Card.EffectSource) and AssignNoCombatDamage's ValidCard$
// (Ophidian's Card.EffectSource) are read; any other mode, key or spelling
// refuses. CanAttackDefender and AssignNoCombatDamage keep their head
// (Card.Self, the spelling the printed S: route lowers). found reports
// whether the body named EffectSource at all.
func effectSourceSelf(s cards.Static) (cards.Static, bool, error) {
	cp := make(map[string]string, len(s.Params))
	found := false
	for k, v := range s.Params {
		if strings.Contains(v, "EffectSource") {
			found = true
			okKey := s.Mode == "CantBlockBy" && k == "ValidAttacker" ||
				s.Mode == "CanAttackDefender" && k == "ValidCard" ||
				s.Mode == "AssignNoCombatDamage" && k == "ValidCard"
			if !okKey || (v != "Creature.EffectSource" && v != "Card.EffectSource") {
				return s, true, refuse("Effect static %s %s$ %q", s.Mode, k, v)
			}
			if s.Mode == "CantBlockBy" {
				v = "Creature.Self"
			} else {
				v = strings.Replace(v, "EffectSource", "Self", 1)
			}
		}
		cp[k] = v
	}
	s.Params = cp
	return s, found, nil
}

// emblemFiltersRefuseHost is emblemFiltersRefuse for a host-bound emblem,
// whose statics read the host as their source: Self is the host, so only
// Other and the chosen type/colour refuse.
func emblemFiltersRefuseHost(where string, fs ...*lFilter) error {
	for _, f := range fs {
		if f == nil {
			continue
		}
		for _, c := range f.Clauses {
			if c.Flags&fOther != 0 || c.Flags&(fChosenType|fChosenColor) != 0 {
				return refuse("%s filter %s reads the host-bound effect's source", where, f)
			}
		}
	}
	return nil
}
