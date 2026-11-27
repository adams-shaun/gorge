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
	if sp.str("Type") != "Spell" {
		return nil, true, refuse("ReduceCost Type %q", sp.str("Type"))
	}
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
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
	readF := sc.readF
	var err error
	caster := sp.str("Caster")
	if caster != "You" && caster != "Player" || sp.str("ValidSA") != "Spell" {
		return nil, true, refuse("CastWithFlash shape")
	}
	sc.st.AnyCaster = caster == "Player"
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if ez, ok := sp.get("EffectZone"); ok {
		// gorge withSelfStatics merges a card's OWN
		// face statics of the mode ONLY when they name an EffectZone$
		// (CR 113.6), so this is the card's own "you may cast this
		// spell as though it had flash if ..." grant from every zone.
		if err := lc.selfFlashShape(s, ez); err != nil {
			return nil, true, err
		}
		lc.out.SelfFlash = true
	}
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

// altGrantShape lowers a battlefield GRANT of a mana alternative cost to the
// controller's own spells (Fist of Suns, Jodah, Leyline of Mutation: bare
// "ValidSA$ Spell | ValidPlayer$ You"; Rooftop Storm's and Runeforge
// Champion's ValidCard$ riders; gorge alternativeCosts' printed battlefield
// walk, read relative to the granter's controller). It reports done with the
// static filled in (kop.StAltGrant, Filter = the ValidCard$); a static of
// another shape is not done and falls through to the own-static arms. Only a
// mana price lowers: the kernel prices the grant as the hand cast's mana
// alternative (dirac/cast_altgrant.go).
func (sc *staticCtx) altGrantShape() (bool, error) {
	sp := sc.sp
	if sp.str("ValidSA") != "Spell" || sp.str("ValidPlayer") != "You" || sp.str("EffectZone") != "" || sp.str("ValidCard") == "Card.Self" {
		return false, nil
	}
	var err error
	if sc.st.F, err = sc.readF("ValidCard"); err != nil {
		return true, err
	}
	if sc.st.Cost, err = sc.lc.altCostCost(sp.str("Cost")); err != nil {
		return true, err
	}
	sc.st.Flags = append(sc.st.Flags, "AltGrant")
	return true, nil
}

// altCostStatic lowers an AlternativeCost static's cost and, for a battlefield
// grant (altGrantShape's "AltGrant" flag), fills in the rest of its static:
// kop.StAltGrant, and Filter = the ValidCard$ the cast card matches. A grant
// carries no non-mana cost part (the kernel prices a grant as the hand cast's
// mana alternative, cast_altgrant.go). A grant's Condition$ lowers through the
// shared static gate into Cond2, which the kernel's altGrantApplies reads with
// the granter as source; none of the five granting cards carries one.
func (cb *cardBuilder) altCostStatic(s lStatic, st *kop.Static) (kop.Cost, error) {
	cost, err := cb.cost(s.Cost)
	if err != nil || !slices.Contains(s.Flags, "AltGrant") {
		return cost, err
	}
	st.StFlags |= kop.StAltGrant
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneHand); err != nil {
			return cost, err
		}
	}
	if cost.Flags != 0 || cost.N != 0 || cost.Mana.X != 0 {
		return cost, fmt.Errorf("alternative cost grant with a non-mana part (the kernel offers mana-only grants)")
	}
	return cost, nil
}
