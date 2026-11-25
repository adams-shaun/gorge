package kgen

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// hexproofOwnColorsKW is the granted keyword lowered to its own static kind
// (kop.StatHexproofOwnColors): the kernel has no free keyword bit, and the
// quality is the recipient's own current colours, not the source's.
const hexproofOwnColorsKW = "Hexproof:CardColors"

// protYourColorsKW is the granted keyword lowered to kop.StatProtYourColors:
// the quality is the colours of the permanents the recipient's controller
// controls, not a filter over the source (gorge rules/protection.go
// protectedFrom, protection_colors_you_ctrl.go).
const protYourColorsKW = "Protection from colorsYouCtrl"

// continuous lowers a Mode$ Continuous static.
func (lc *lowerCtx) continuous(sp *params, st *lStatic) ([]lStatic, error) {
	aff := sp.str("Affected")
	if err := lc.grantedLookOnly(sp); err != nil {
		return nil, err
	}
	if aff == "" {
		if !sp.is("CharacteristicDefining", "True") {
			return nil, refuse("Continuous without Affected")
		}
		aff = "Card.Self"
	}
	if _, ok := sp.m["SharedKeywordsZone"]; ok {
		return lc.sharedKeywordStatics(sp, st) // lower_sharedkw.go
	}
	if kw := sp.str("AddKeyword"); kw == hexproofOwnColorsKW {
		// Tam, Mindful First-Year's "each other creature you control has
		// hexproof from each of its colors": the exact Affected$ shape only,
		// and nothing beside it (a condition or any unread key refuses).
		if aff != "Creature.Other+YouCtrl" {
			return nil, refuse("Hexproof:CardColors Affected %q", aff)
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("Hexproof:CardColors static with a condition")
		}
		f, err := lc.filter(aff)
		if err != nil {
			return nil, err
		}
		st.F = f
		st.Kind = "HexproofOwnColors"
		return nil, nil
	}
	if kw := sp.str("AddKeyword"); kw == protYourColorsKW {
		// Empty-Shrine Kannushi's self grant: the exact Affected$ shape only,
		// and nothing beside it (a condition or any unread key refuses).
		if aff != "Card.Self" {
			return nil, refuse("%s Affected %q", protYourColorsKW, aff)
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("%s static with a condition", protYourColorsKW)
		}
		f, err := lc.filter(aff)
		if err != nil {
			return nil, err
		}
		st.F = f
		st.Kind = "ProtYourColors"
		return nil, nil
	}
	// the view-only top-of-library look. gorge reads
	// MayLookAt$ You/True/Player into a look-only continuous effect
	// (engine/rules/layers.go:766-785) whose only consumers are
	// Engine.MayLookAtLibraryTop (engine/rules/layers_static.go:253-330)
	// and the seat view (engine/view/view.go:66-72, :715): no event,
	// decision or legality reader at the pin. Lower it to nothing, and only
	// for the exact library-top shape -- every other MayLookAt$ (hand, a
	// library+battlefield mix, or the MayPlay permission beside it) stays
	// refused.
	//
	// and the view-only face-down battlefield look
	// (DSK Found Footage's printed static, MKM Lumbering Laundry's Effect
	// static). Its readers are the look reads (MayLookAtLibraryTop for the
	// library top; MayLookAtFaceDown for exile and, since cr-g303, a
	// face-down battlefield permanent whose grant names Battlefield), the
	// seat view (engine/view/card.go) and the oracle runner's expectation
	// read; no event, decision or legality reader changes state, so it too
	// lowers to no static. faceDownLookOnly is the exact shape: any
	// other MayLookAt$ spelling -- in particular one beside MayPlay$ --
	// keeps the named refusal below.
	if _, ok := sp.get("MayLookAt"); ok {
		return nil, lc.libTopLook(sp, st, aff)
	}
	if kw, _ := sp.get("AddKeyword"); kw == "Beam me up" {
		return nil, lc.beamGrant(sp, st, aff) // lower_beammeup.go
	}
	if az, ok := sp.get("AffectedZone"); ok {
		extra, done, err := lc.affectedZone(sp, st, aff, az) // lower_offzone.go
		if err != nil {
			return nil, err
		}
		if done {
			return extra, nil
		}
	}
	if sp.is("MayPlay", "True") {
		return nil, lc.mayPlayStatic(sp, st, aff)
	}
	if _, ok := sp.get("SetMaxHandSize"); ok && aff == "Opponent" {
		return nil, lc.oppMaxHand(sp, st) // lower_playerstatic.go
	}
	if aff == "You" {
		ok, err := lc.youPlayerStatic(sp, st)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, refuse("player static")
		}
		return nil, nil
	}
	if hk, ok := sp.m["AddHiddenKeyword"]; ok && !sp.used["AddHiddenKeyword"] { // a lure grant lureGrant took is lowered beside
		sp.used["AddHiddenKeyword"] = true
		if strings.TrimSpace(hk) != "CARDNAME can't block." {
			return nil, refuse("AddHiddenKeyword %q", hk)
		}
		f, err := lc.filter(aff)
		if err != nil {
			return nil, err
		}
		st.F = f
		st.Kind = "CantBlock"
		return nil, nil
	}
	f, err := lc.filter(aff)
	if err != nil {
		return nil, err
	}
	st.F = f
	if gc, ok := sp.get("GainControl"); ok {
		// layer 2 (CR 613.1b): "You control enchanted creature";
		// the kernel settles it on the source's attached object, so the
		// affected set must be that object and the static unconditional
		// (any further effect param stays unread and refuses)
		if gc != "You" {
			return nil, refuse("GainControl static %q", gc)
		}
		if !attachedOnly(f) {
			return nil, refuse("GainControl static affected %s", f)
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("GainControl static with a condition")
		}
		st.Kind = "Control"
		return nil, nil
	}
	if sp.is("CharacteristicDefining", "True") {
		folded, err := lc.cdaColorType(sp, st, aff) // lower_cdacolortype.go
		if err != nil {
			return nil, err
		}
		if _, p := sp.m["SetPower"]; folded && !p && sp.m["SetToughness"] == "" {
			st.Kind = "" // the printed record carries it: lower to nothing
			return nil, nil
		}
		// gorge CDASetPT: SetPower$/SetToughness$ each set its half; a
		// missing half keeps the printed value (the "*/4" cards)
		pw, tg, _ := strings.Cut(lc.face.PT, "/")
		half := func(k, printed string) (lValue, error) {
			if e, ok := sp.get(k); ok {
				return lc.value(e)
			}
			n, err := strconv.Atoi(printed)
			if err != nil {
				return lValue{}, refuse("CDA shape")
			}
			return cst(n), nil
		}
		sp2, ok1 := sp.m["SetPower"]
		st2, ok2 := sp.m["SetToughness"]
		if !ok1 && !ok2 {
			return nil, refuse("CDA shape")
		}
		if ok1 && ok2 && sp2 == st2 {
			v, err := lc.value(sp.str("SetPower"))
			if err != nil {
				return nil, err
			}
			sp.consume("SetToughness")
			if ptAggIn(v) {
				return nil, refuse("CDA reads a P/T aggregate") // the layer walk would recurse
			}
			st.Kind = "CDAPT"
			st.Power, st.Tough = v, v
			return nil, nil
		}
		p, err := half("SetPower", pw)
		if err != nil {
			return nil, err
		}
		t, err := half("SetToughness", tg)
		if err != nil {
			return nil, err
		}
		if ptAggIn(p) || ptAggIn(t) {
			return nil, refuse("CDA reads a P/T aggregate") // the layer walk would recurse
		}
		st.Kind = "CDAPT"
		st.Power, st.Tough = p, t
		return nil, nil
	}
	// a Station threshold's self grant lowers here; the rest of the static
	// (AddType, AddKeyword) lowers as any Station threshold static
	if err := lc.stationSelfGrant(sp, st); err != nil {
		return nil, err
	}
	hard := false
	for _, k := range []string{"SetPower", "SetToughness", "SetColor", "RemoveAllAbilities", "RemoveCardTypes", "RemoveCreatureTypes", "RemoveType", "SetName", "AddAllCreatureTypes", "RemoveLandTypes"} {
		if _, ok := sp.m[k]; ok {
			hard = true
		}
	}
	if t, ok := sp.get("AddType"); ok && !hard {
		// a card type added by a static (Ygra's "are Food artifacts in
		// addition to their other types") is a layer-4 override (CR 613);
		// subtypes alone stay a Continuous subtype grant
		for _, w := range strings.Split(t, "&") {
			if _, ok := typeAddBit(strings.TrimSpace(w)); ok {
				hard = true
			}
		}
	}
	if hard {
		return lc.overrideStatic(sp, st)
	}
	if r, ok := sp.get("RemoveKeyword"); ok {
		// a layer-6 removal beside the Continuous parts (Starforged
		// Sword's "gets +3/+3 and loses flying"): an Override carrying it
		ks, err := removeKeywords(r)
		if err != nil {
			return nil, err
		}
		st.Override = &lOverride{RemoveKw: ks}
		lc.out.overrides = append(lc.out.overrides, st.Override)
		if err := cantHaveKeyword(sp, st); err != nil {
			return nil, err // lower_canthavekw.go
		}
	}
	if err := lc.continuousPT(sp, st); err != nil {
		return nil, err
	}
	if k, ok := sp.get("AddKeyword"); ok {
		if aff == "You" {
			return nil, refuse("player keyword")
		}
		// gorge playerkeywords.go's playerKeywords surface reads only the
		// Shroud and Hexproof AddKeyword$ grants; any other keyword granted
		// to a PLAYER spec is a ContinuousEffect nothing reads (the object
		// keyword readers' player grammar fails closed, and the grant is
		// gorge's granted-keyword-gaps ratchet: the card is playable while
		// the grant is inert; the LimitSearchLibrary line in gorge's
		// engine/rules/testdata/granted-keyword-gaps.txt records the gap, and
		// this lowering goes when gorge models the grant). When the grant is the static's whole effect it
		// lowers to nothing, exactly gorge's behaviour. Aven Mindcensor's
		// "if an opponent would search a library, that player searches the
		// top four cards instead" -- gorge models neither the search limit
		// nor any other reader for the grant.
		if inertKeywordGrant(sp, st, k) { // lower_grantedsoulshift.go: the grant lowers to nothing
			return nil, nil
		}
		rest, ward, wardLife, err := splitWardGrant(k)
		if err != nil {
			return nil, err
		}
		rest, fbs, err := splitFirebendingGrant(rest)
		if err != nil {
			return nil, err
		}
		rest, melee := splitMeleeGrant(rest)
		rest, exalted := splitExaltedGrant(rest)
		rest, err = lc.grantEvolve(rest, st) // lower_evolvegrant.go
		if err != nil {
			return nil, err
		}
		rest, err = lc.grantMentor(rest, st) // lower_mentorgrant.go
		if err != nil {
			return nil, err
		}
		rest, saddles := splitSaddleGrant(rest)            // lower_saddlegrant.go
		rest, livingWeapon := splitLivingWeaponGrant(rest) // lower_livingweapongrant.go
		rest, extort := splitExtortGrant(rest)             // lower_extortgrant.go
		rest, myriad := splitMyriadGrant(rest)             // lower_myriad.go
		rest, err = lc.splitProtGrant(rest, st)            // lower_protgrant.go
		if err != nil {
			return nil, err
		}
		rest, st.Landwalk, err = lc.splitLandwalkGrant(rest, aff) // lower_landwalkgrant.go
		if err != nil {
			return nil, err
		}
		rest, st.Evasion, st.Infect = splitStaticKwMarks(rest) // lower_kwmark.go
		if rest, err = lc.staticBandingGrant(sp, st, rest); err != nil {
			return nil, err // lower_banding.go
		}
		if rest != "" {
			if st.Kw, err = pumpKeywords(rest); err != nil {
				return nil, err
			}
		}
		for _, n := range fbs {
			// CR 702.189a granted: one latent Attacks trigger per grant
			// (lower_firegrant.go; gorge trigger_granted.go)
			idx, err := lc.firebendGrantTrigger(n)
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; melee > 0; melee-- {
			// CR 702.121a/b granted: one latent Attacks trigger, listed
			// once per instance (lower_meleegrant.go; gorge rules/melee.go)
			idx, err := lc.meleeGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; exalted > 0; exalted-- {
			// CR 702.83a-b granted: one latent Attacks trigger, listed
			// once per instance (lower_exaltedgrant.go; gorge
			// rules/melee.go)
			idx, err := lc.exaltedGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for _, line := range saddles {
			// CR 702.171a granted: one latent AB$ AlterAttribute per grant
			// (lower_saddlegrant.go; gorge cards.GrantedKeywordAbility)
			idx, err := lc.saddleGrantAbility(line)
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; livingWeapon > 0; livingWeapon-- {
			// CR 702.92a granted: one latent ChangesZone Self trigger, listed
			// once per instance (lower_livingweapongrant.go; gorge
			// scriptfacts.GrantedKeywordTrigger)
			idx, err := lc.livingWeaponGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; extort > 0; extort-- {
			// CR 702.101a/b granted: one latent SpellCast trigger, listed
			// once per instance (lower_extortgrant.go; gorge
			// rules/scriptfacts)
			idx, err := lc.extortGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; myriad > 0; myriad-- {
			// CR 702.116a-b granted: one latent Attacks Self trigger,
			// listed once per instance (lower_myriad.go; gorge
			// rules/trigger_granted.go)
			idx, err := lc.myriadGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		if ward > 0 || wardLife > 0 {
			// CR 702.21a: a granted ward {N} or ward—pay N life (Sheltered by
			// Ghosts' Aura, Coppercoat Vanguard's anthem): the KwWard bit plus
			// the static's own amount (kop.Static.Ward / WardLife; kernel
			// kw_ward.go)
			st.Kw = append(st.Kw, "Ward")
			st.Ward = ward
			st.WardLife = wardLife
		}
	}
	if t, ok := sp.get("AddType"); ok {
		for _, w := range strings.Split(t, "&") {
			w = strings.TrimSpace(w)
			if w == "ChosenType" {
				// gorge resolveChosenTypes: the host's recorded choice, live;
				// none recorded grants nothing
				st.AddChosen = true
				continue
			}
			if w == "AllBasicLandType" {
				// gorge appendLandTypes: the five basic land types in order,
				// each once (appendTypeOnce); the kernel's mana walk adds the
				// CR 305.6 intrinsic ability per granted type
				for _, b := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
					if !slices.Contains(st.AddSub, b) {
						st.AddSub = append(st.AddSub, b)
					}
				}
				continue
			}
			if !lc.known(w) {
				return nil, refuse("AddType %q", w)
			}
			if !slices.Contains(st.AddSub, w) {
				st.AddSub = append(st.AddSub, w)
			}
		}
	}
	if err := lc.abilityGrants(sp, st, aff); err != nil {
		return nil, err
	}
	if tr, ok := sp.m["AddTrigger"]; ok && !sp.used["AddTrigger"] { // not already lowered by stationSelfGrant
		sp.used["AddTrigger"] = true
		idx, err := lc.grantedTrigger(sp, st, tr)
		if err != nil {
			return nil, err
		}
		st.Grant = append(st.Grant, idx)
	}
	if readsFlying(st.F) {
		// a keyword-gated layer-6 effect is ordered after the effects it
		// depends on (gorge abilityDependencyOrder); one that changes the
		// keyword it reads could form a dependency cycle
		for _, k := range st.Kw {
			if k == "Flying" {
				return nil, refuse("keyword-gated static grants the keyword it reads")
			}
		}
		if st.Override != nil {
			for _, k := range st.Override.RemoveKw {
				if k == "Flying" {
					return nil, refuse("keyword-gated static removes the keyword it reads")
				}
			}
		}
	}
	return nil, nil
}

// abilityGrants lowers a Continuous static's activated-ability grants: its
// GainsAbilitiesOf$ (lower_gainsof.go) and its AddAbility$ (unless
// stationSelfGrant already lowered it).
func (lc *lowerCtx) abilityGrants(sp *params, st *lStatic, aff string) error {
	if err := lc.gainsAbilitiesOf(sp, st, aff); err != nil {
		return err
	}
	if ab, ok := sp.m["AddAbility"]; ok && !sp.used["AddAbility"] { // not already lowered by stationSelfGrant
		sp.used["AddAbility"] = true
		idx, err := lc.grantedAbility(ab)
		if err != nil {
			return err
		}
		st.Grant = append(st.Grant, idx)
	}
	return nil
}

// removeKeywords lowers a RemoveKeyword$ list: plain keywords only (a
// parameterised one, Landwalk:Island, has no kernel bit).
func removeKeywords(kw string) ([]string, error) {
	var out []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if !simpleKeywords[k] {
			return nil, refuse("RemoveKeyword %q", k)
		}
		out = append(out, k)
	}
	return out, nil
}

// readsFlying reports whether a filter reads keywords (withFlying /
// withoutFlying, the only keyword reads kgen lowers).
func readsFlying(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&(fWithFlying|fWithoutFlying) != 0 {
			return true
		}
	}
	return false
}

// grantedTrigger lowers a static's AddTrigger$ (gorge layer 6: the
// affected permanent has the triggered ability; its own controller
// controls the trigger and Self is that permanent). The kernel's static
// grant walk (grantedAbilitiesN) reports the grant for an attached host (an
// Equipment's or Aura's) and, for any other affected filter, for every
// permanent the filter matches (globalGrantsGen: "artifact creatures you
// control have ..."), one instance per granting static in timestamp order.
// AddSVar$
// names SVars of this face the granted body reads (consumed).
func (lc *lowerCtx) grantedTrigger(sp *params, st *lStatic, name string) (int, error) {
	lc.granted++
	defer func() { lc.granted-- }()
	// An attached grant names its host: the kernel finds it as the card's own
	// permanent attached to the carrier (cost_sachost.go).
	aff := sp.str("Affected") // AffectedDefined$ Equipped/Enchanted arrives as .EquippedBy/.EnchantedBy
	lc.grantAttached = !strings.Contains(aff, ",") && (strings.Contains(aff, ".EquippedBy") || strings.Contains(aff, ".EnchantedBy"))
	defer func() { lc.grantAttached = false }()
	if st.F == nil || st.F.Players != 0 || len(st.F.Clauses) == 0 {
		return 0, refuse("AddTrigger affected %s", st.F)
	}
	if strings.Contains(name, "&") || strings.Contains(name, ",") {
		return 0, refuse("AddTrigger %q: more than one trigger", name)
	}
	if sv, ok := sp.get("AddSVar"); ok {
		for _, n := range strings.FieldsFunc(sv, func(r rune) bool { return r == '&' || r == ',' }) {
			if _, ok := lc.face.SVars[strings.TrimSpace(n)]; !ok {
				return 0, refuse("AddSVar %q not on the face", n)
			}
		}
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return 0, refuse("AddTrigger %q", name)
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	if t.Mode == "Blocks" || t.Mode == "AttackerBlocked" || t.Mode == "AttackerBlockedByCreature" {
		return 0, refuse("AddTrigger of mode %s", t.Mode)
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) < idx+1 {
		return 0, refuse("AddTrigger lowering")
	}
	// (an ImmediateTrigger body appends its reflexive ability after the
	// granted one; it is reached through the body's own spawn, not granted)
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = "Granted"
	return idx, nil
}

// splitWardGrant takes a Continuous static's "Ward:N" or "Ward:PayLife<N>"
// term (one) out of an AddKeyword$ list: the rest of the list and the mana N
// or life N (0: none). A ward with another cost or two ward terms are
// refused.
func splitWardGrant(kw string) (string, int, int, error) {
	var rest []string
	ward, wardLife := 0, 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		v, ok := strings.CutPrefix(k, "Ward:")
		if !ok {
			rest = append(rest, k)
			continue
		}
		if life, ok := strings.CutPrefix(v, "PayLife<"); ok {
			body, ok := strings.CutSuffix(life, ">")
			if !ok {
				return "", 0, 0, refuse("keyword %q", k)
			}
			n, err := strconv.Atoi(body)
			if err != nil || n < 1 || n > 255 || ward != 0 || wardLife != 0 {
				return "", 0, 0, refuse("keyword %q", k)
			}
			wardLife = n
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 255 || ward != 0 || wardLife != 0 {
			return "", 0, 0, refuse("keyword %q", k)
		}
		ward = n
	}
	return strings.Join(rest, " & "), ward, wardLife, nil
}

// continuousPT lowers AddPower$/AddToughness$ and marks the AffectedX
// convention: an amount whose value is the AffectedX SVar reads each AFFECTED
// object as its source (gorge layers_static.go staticAmountOn's affected
// anchor, effects.AffectedXStaticAmount), so the static carries the
// "PerAffected" flag and the walk re-anchors the eval context (kop.
// StPerAffected). An inline count expression stays anchored on the grantor
// on both sides. StPerAffected is ONE flag for both amounts, while gorge
// anchors per amount (engine/rules/chars/derive.go reads AddPowerAffected and
// AddToughnessAffected separately), so a mixed shape -- one amount AffectedX,
// the other a dynamic grantor read -- has no shared lowering: the affected
// creature would feed the grantor's count. It is refused (fail-closed;
// StaticFlags has no free bit for a per-amount pair), while a mixed shape
// whose other amount is a constant lowers (the anchor is irrelevant to a
// constant).
func (lc *lowerCtx) continuousPT(sp *params, st *lStatic) error {
	var pAff, tAff bool
	if p, ok := sp.get("AddPower"); ok {
		v, err := lc.value(p)
		if err != nil {
			return err
		}
		st.Power = v
		pAff = effects.AffectedXStaticAmount(p)
		st.PerAffected = st.PerAffected || pAff
	}
	if t, ok := sp.get("AddToughness"); ok {
		v, err := lc.value(t)
		if err != nil {
			return err
		}
		st.Tough = v
		tAff = effects.AffectedXStaticAmount(t)
		st.PerAffected = st.PerAffected || tAff
	}
	if st.PerAffected {
		// exactly one amount is AffectedX: the other amount, if it is a
		// dynamic value, would be re-anchored at the affected object while
		// gorge keeps it on the grantor
		pDyn := st.Power.Kind != vNone && !st.Power.isConst()
		tDyn := st.Tough.Kind != vNone && !st.Tough.isConst()
		if pAff != tAff && ((pAff && tDyn) || (tAff && pDyn)) {
			return refuse("mixed AffectedX P/T: StPerAffected re-anchors both amounts (AddPower$ %s, AddToughness$ %s)", st.Power, st.Tough)
		}
	}
	return nil
}
