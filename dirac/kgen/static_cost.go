package kgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The bodies of the cost and casting statics (static_mode.go).
func init() {
	staticMode("ReduceCost", (*staticCtx).modeReduceCost)
	staticMode("RaiseCost", (*staticCtx).modeRaiseCost)
	staticMode("CastWithFlash", (*staticCtx).modeCastWithFlash)
	staticMode("AlternativeCost", (*staticCtx).modeAlternativeCost)
	staticMode("Activations", (*staticCtx).modeActivations)
	staticMode("ManaConvert", (*staticCtx).modeManaConvert)
}

func (sc *staticCtx) modeReduceCost() ([]lStatic, bool, error) {
	lc := sc.lc
	s := sc.s
	sp := sc.sp
	readF := sc.readF
	var err error
	if sp.str("Type") == "Ability" || sp.str("Type") == "" && (strings.HasPrefix(sp.str("ValidSpell"), "Activated.") || sp.str("ValidSpell") == "Static.Unlock") {
		if err := lc.reduceAbility(sp, &sc.st, readF); err != nil {
			return nil, true, err
		}
		return nil, false, nil // lower_reduceab.go
	}
	faceDownCast := sp.str("Type") == "" && strings.TrimSpace(sp.str("ValidSpell")) == "Spell.isCastFaceDown"
	if sp.str("Type") != "Spell" && !faceDownCast {
		return nil, true, refuse("ReduceCost Type %q", sp.str("Type"))
	}
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if faceDownCast {
		// "Face-down creature spells you cast cost {1} less" (Obscuring
		// Aether): ValidCard$ is the creature filter, the ValidSpell$
		// word is the cast's face-down predicate (gorge statics_cost's
		// spellConstraintMatchesIsCastFaceDown, kop.PredCastFaceDown).
		sp.consume("ValidSpell")
		for i := range sc.st.F.Clauses {
			if sc.st.F.Clauses[i].CmpStat != cmpNone {
				return nil, true, refuse("ReduceCost Spell.isCastFaceDown beside another filter predicate")
			}
			sc.st.F.Clauses[i].CmpStat = cmpCastFaceDown
		}
	}
	if opt, err := lc.reduceOptPaid(sp, &sc.st); err != nil {
		return nil, true, err // lower_optcost.go
	} else if !opt {
		if sp.str("ValidCard") == "Card.Self" {
			sp.consume("Relative") // gorge lists Relative$ as readable and never reads it (as reduceOptPaid)
		}
		if sc.st.Amount, err = lc.value(sp.str("Amount")); err != nil {
			return nil, true, err
		}
	}
	if a, ok := sp.get("Activator"); ok && a != "You" {
		return nil, true, refuse("ReduceCost Activator %q", a)
	} else if !ok && sp.str("ValidCard") != "Card.Self" {
		// no Activator$: either player's spells (CR 601.2f; Forge
		// StaticAbilityCostChange applies it to every payer)
		sc.st.Flags = append(sc.st.Flags, "AnyCaster")
	}
	if ez, ok := sp.get("EffectZone"); ok {
		if ez != "All" {
			return nil, true, refuse("EffectZone %q", ez)
		}
		sc.st.Zone = zAny
	}
	lc.spellBaseOK = true
	sc.st.F2, err = readF("ValidTarget")
	lc.spellBaseOK = false
	if err != nil {
		return nil, true, err
	}
	if sc.st.F == nil {
		// OnlyFirstSpell$ without ValidCard$ (Melek): the in-flight
		// gate is the ValidSpell$ filter, which keeps its Spell head
		// (build_ab builds st.F at ZoneStack); the first-spell ledger
		// spec is the stripped card spec (reduceGates).
		lc.spellBaseOK = true
		sc.st.F, err = readF("ValidSpell")
		lc.spellBaseOK = false
		if err != nil {
			return nil, true, err
		}
	}
	if err := lc.reduceGates(sp, s.Params, &sc.st); err != nil {
		return nil, true, err // lower_reducegates.go
	}
	if err := lc.reduceSpellShape(sp, &sc.st); err != nil {
		return nil, true, err // lower_reducespell.go
	}
	return nil, false, nil
}

func (sc *staticCtx) modeRaiseCost() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if done, err := lc.raiseAbility(sp, &sc.st); err != nil {
		return nil, true, err // static_raiseability.go
	} else if done {
		return nil, false, nil
	}
	done, err := lc.raiseSelfWaterbend(sp, &sc.st) // lower_raisewaterbend.go
	if err == nil && !done {
		done, err = lc.raiseSelfAdditional(sp) // lower_raisecost.go
	}
	if err == nil && !done {
		done, err = lc.raiseSelfBehold(sp, &sc.st)
	}
	if err != nil {
		return nil, true, err
	} else if !done {
		if err := lc.raiseCost(sp, &sc.st); err != nil {
			return nil, true, err
		}
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCastWithFlash() ([]lStatic, bool, error) {
	lc := sc.lc
	s := sc.s
	sp := sc.sp
	if sp.str("ValidSA") == "Activated.Equip" {
		if err := sc.equipFlashShape(); err != nil { // static_equipflash.go
			return nil, true, err
		}
		return nil, false, nil
	}
	if sp.str("ValidSA") == "Activated.Loyalty" {
		return sc.loyaltyFlash() // static_loyaltyflash.go
	}
	caster := sp.str("Caster")
	validSA := sp.str("ValidSA")
	tgtSpec, isTgt := selfFlashTargeting(validSA)
	_, hasZone := sp.get("EffectZone")
	if caster != "You" && caster != "Player" || isTgt && !hasZone {
		return nil, true, refuse("CastWithFlash shape")
	}
	var xmax int
	var subtype string
	var err error
	if !isTgt {
		xmax, subtype, err = cwfValidSA(validSA) // static_cwfvalidsa.go
		if err != nil {
			return nil, true, err
		}
	}
	sc.st.AnyCaster = caster == "Player"
	if sc.st.F, err = sc.cwfValidCard(subtype); err != nil {
		return nil, true, err
	}
	if ez, ok := sp.get("EffectZone"); ok {
		// gorge withSelfStatics merges a card's OWN
		// face statics of the mode ONLY when they name an EffectZone$
		// (CR 113.6), so this is the card's own "you may cast this
		// spell as though it had flash if ..." grant from every zone.
		if err := lc.selfFlashShape(s, ez, validSA); err != nil {
			return nil, true, err
		}
		if isTgt {
			// ValidSA$ Spell.IsTargeting Valid <spec> (Flash Photography,
			// Timely Ward): the grant holds only while the spell can
			// announce a target matching <spec> (lower_static.go)
			if err := lc.selfFlashTargetsFilter(&sc.st, tgtSpec); err != nil {
				return nil, true, err
			}
		}
		lc.out.SelfFlash = true
	} else if xmax != 0 || validSA == "Spell.Teamwork" {
		// the kernel bounds only the SelfFlash carrier's own X
		// (dirac/cast_flashxcap.go), and the Teamwork permission is the
		// card's own self grant: a battlefield static granting it to others
		// is not a kernel shape
		return nil, true, refuse("CastWithFlash shape")
	}
	sc.st.FlashXMax = xmax
	return nil, false, nil
}

// no EffectZone$: withSelfStatics skips the static in hand, so it
// grants only from the battlefield and never its own card flash
// (Captain Mar-Vell): an ordinary battlefield static
func (sc *staticCtx) modeAlternativeCost() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	var err error
	if sp.str("ValidSA") == "Activated.Crew+Vehicle" {
		return sc.crewAltCost() // static_crewalt.go
	}
	if sp.str("ValidSA") == "Activated.Cycling" {
		return sc.modeCyclingAlt() // static_cyclingalt.go
	}
	if sp.str("ValidSA") == "Spell" && sp.str("ValidCard") == "Card.Self" && sp.str("ValidPlayer") == "You" && sp.str("EffectZone") == "All" {
		// (Land Grant) gorge alternativeCosts' face walk
		// prices a card's own static; bare ValidSA$ Spell, its own
		// ValidCard$ and ValidPlayer$ You is the Spell.Self grant
		// (ValidCard$ is not read on that walk).
		sp.consume("ValidCard", "ValidPlayer")
		sp.m["ValidSA"] = "Spell.Self"
	}
	if done, err := sc.altGrantShape(); done || err != nil {
		return nil, err != nil, err
	}
	if sp.str("ValidSA") != "Spell.Self" || !altCostZoneOK(sp.str("EffectZone")) {
		return nil, true, refuse("AlternativeCost shape")
	}
	// EffectZone$ Graveyard names the graveyard explicitly: the zone the
	// lowered static carries, the kernel's one home for the hand walk's
	// exclusion (offerAltFreeHand) and the graveyard offer's arm
	// (offerGraveyardAltCasts). gorge graveyardSelfAltCost: an EffectZone$
	// All one admits the graveyard too but is never a graveyard cast.
	if sp.str("EffectZone") == "Graveyard" {
		sc.st.Zone = zGraveyard
	}
	if sp.str("Cost") == "Reveal<1/Hand>" {
		// "reveal your hand" (gorge cost.go
		// IsWholeHandRevealSpec: payable with any hand, no choice) is
		// a free alternative cost (kernel CardDef.AltFree)
		sc.st.Cost = lCost{}
		sp.consume("Cost")
	} else if sc.st.Cost, err = lc.altCostCost(sp.str("Cost")); err != nil {
		return nil, true, err
	}
	// a Return<N/Spec> part lowers alone (Gush, Thwart, Daze: the
	// kernel's return alternative, build_ab.go altReturnOK); beside any
	// other part build_ab.go refuses it as a non-mana alternative
	if sc.st.Cost.Forage != nil || sc.st.Cost.ExileG != nil && !sc.st.Cost.ExileHand && !altGraveExileCost(&sc.st.Cost) || sc.st.Cost.Energy != 0 {
		return nil, true, refuse("alternative cost with a forage / exile-from-graveyard part")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeActivations() ([]lStatic, bool, error) {
	s := sc.s
	sp := sc.sp
	if err := activationLimitStatic(s, sp, &sc.st); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}

func (sc *staticCtx) modeManaConvert() ([]lStatic, bool, error) {
	if err := sc.manaConvertStatic(); err != nil {
		return nil, true, err // lower_manaconvert.go
	}
	return nil, false, nil
}

// altCostZoneOK is a printed AlternativeCost static's EffectZone$ the kernel
// prices: All (the hand walk's grant, the Land Grant and Force cycle shapes)
// or Graveyard (the self-carried graveyard cast, T18b-B's four).
func altCostZoneOK(ez string) bool {
	return ez == "All" || ez == "Graveyard"
}

// flashCasterFlag is a CastWithFlash static's Static.StFlags caster bit.
func (s lStatic) flashCasterFlag() kop.StaticFlags {
	if s.AnyCaster {
		return kop.StFlashAnyCaster
	}
	return 0
}

// altGrantShape lowers a battlefield GRANT of an alternative cost to the
// controller's own spells (Fist of Suns, Jodah, Leyline of Mutation: bare
// "ValidSA$ Spell | ValidPlayer$ You"; Rooftop Storm's and Runeforge
// Champion's ValidCard$ riders; gorge alternativeCosts' printed battlefield
// walk, read relative to the granter's controller), or to any player's spells
// when no ValidPlayer$ rider scopes it (Dream Halls' discard alternative). A
// spell filter in ValidSA$ (Kentaro's "Spell.Samurai") is the grant's filter
// when XAlternative$ prices its {X} at the cast card's mana value. It reports
// done with the static filled in (kop.StAltGrant, Filter = the ValidCard$ or
// the ValidSA$ filter); a static of another shape is not done and falls
// through to the own-static arms. A mana price lowers, and so does Dream
// Halls' dynamic colour-share discard (altGrantDiscardCost).
func (sc *staticCtx) altGrantShape() (bool, error) {
	sp := sc.sp
	vs := sp.str("ValidSA")
	vp := sp.str("ValidPlayer")
	xalt := sp.str("XAlternative")
	var xf *lFilter
	switch {
	case vs == "Spell":
	case xalt != "" && strings.HasPrefix(vs, "Spell.") && vs != "Spell.Self":
		// Kentaro, the Smiling Cat: the ValidSA$ constraint after Spell. is
		// the cast card's filter, and XAlternative$ Count$CardManaCost
		// prices the grant's one {X} (the kernel substitutes the cast
		// card's mana value; cast_altgrant.go).
		if xalt != "Count$CardManaCost" {
			return true, refuse("AlternativeCost XAlternative %q", xalt)
		}
		f, err := sc.lc.filter(strings.TrimPrefix(vs, "Spell."))
		if err != nil {
			return true, err
		}
		xf = f
	default:
		return false, nil
	}
	if vp != "You" && vp != "" || sp.str("EffectZone") != "" || sp.str("ValidCard") == "Card.Self" {
		return false, nil
	}
	if xf != nil && sp.str("ValidCard") != "" {
		return true, refuse("AlternativeCost XAlternative beside ValidCard$")
	}
	var err error
	if xf != nil {
		sc.st.F = xf
	} else if sc.st.F, err = sc.readF("ValidCard"); err != nil {
		return true, err
	}
	if sc.st.Cost, err = sc.lc.altCostCost(sp.str("Cost")); err != nil {
		return true, err
	}
	if xf != nil {
		if !xaltCostShape(&sc.st.Cost) {
			return true, refuse("XAlternative cost %q: want one {X} and nothing else", sp.str("Cost"))
		}
		sc.st.AltXMana = true
	}
	// A grant with no ValidPlayer$ rider applies to every player's spell
	// (Dream Halls: "its controller may discard..."), not only the static's
	// controller's (kop.StAltAnyCaster).
	sc.st.AnyCaster = vp == ""
	sc.st.Flags = append(sc.st.Flags, "AltGrant")
	return true, nil
}

// xaltCostShape is an XAlternative$ Count$CardManaCost price: one {X} and no
// other mana or non-mana part (Kentaro's Cost$ X). The kernel replaces that X
// with the cast card's mana value (dirac/cast_altgrant.go).
func xaltCostShape(c *lCost) bool {
	m := c.Mana
	return c.pureMana() && m.X && !m.XX && !m.XXX && m.Generic == 0 &&
		m.Pips == [5]int{} && len(m.Hybrid) == 0 && len(m.Twobrid) == 0 &&
		len(m.Phyrexian) == 0 && m.C == 0 && m.Snow == 0 && m.XColor == 0 && !m.None
}

// altGrantDiscardCost reports whether a grant's cost is Dream Halls' dynamic
// discard: one hand card sharing a colour with the spell being cast, with no
// fixed filter and no other part (kop.Cost.DiscardSharesCastColor).
func altGrantDiscardCost(c *kop.Cost) bool {
	return *c == kop.Cost{Flags: kop.CostDiscard, N: 1, DiscardSharesCastColor: true}
}

// altCostStatic lowers an AlternativeCost static's cost and, for a battlefield
// grant (altGrantShape's "AltGrant" flag), fills in the rest of its static:
// kop.StAltGrant, kop.StAltAnyCaster for a grant with no ValidPlayer$ rider,
// and Filter = the ValidCard$ the cast card matches. A grant carries either a
// mana price (the kernel prices it as the hand cast's mana alternative,
// cast_altgrant.go) or Dream Halls' dynamic discard pick (altGrantDiscardCost).
// A grant's Condition$ lowers through the shared static gate into Cond2, which
// the kernel's altGrantApplies reads with the granter as source; none of the
// granting cards carries one.
func (cb *cardBuilder) altCostStatic(s lStatic, st *kop.Static) (kop.Cost, error) {
	cost, err := cb.cost(s.Cost)
	if err != nil || !slices.Contains(s.Flags, "AltGrant") {
		return cost, err
	}
	st.StFlags |= kop.StAltGrant
	if s.AnyCaster {
		st.StFlags |= kop.StAltAnyCaster
	}
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneHand); err != nil {
			return cost, err
		}
	}
	if s.AltXMana {
		// the grant's one {X} is the cast card's mana value (Kentaro); any
		// other mana or a non-mana part is not that shape
		if cost.Mana.X != 1 || cost.Mana.MV() != 0 || cost.Flags != 0 || cost.N != 0 {
			return cost, fmt.Errorf("X alternative cost grant with a non-mana part or a non-bare {X} price")
		}
		return cost, nil
	}
	if cost.Flags != 0 || cost.N != 0 || cost.Mana.X != 0 {
		if altGrantDiscardCost(&cost) {
			return cost, nil
		}
		return cost, fmt.Errorf("alternative cost grant with a non-mana part (the kernel offers mana grants and Dream Halls' colour-share discard)")
	}
	return cost, nil
}
