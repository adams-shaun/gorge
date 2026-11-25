package kgen

import (
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
	if sp.str("ValidSA") == "Spell" && sp.str("ValidCard") == "Card.Self" && sp.str("ValidPlayer") == "You" && sp.str("EffectZone") == "All" {
		// (Land Grant) gorge alternativeCosts' face walk
		// prices a card's own static; bare ValidSA$ Spell, its own
		// ValidCard$ and ValidPlayer$ You is the Spell.Self grant
		// (ValidCard$ is not read on that walk).
		sp.consume("ValidCard", "ValidPlayer")
		sp.m["ValidSA"] = "Spell.Self"
	}
	if sp.str("ValidSA") != "Spell.Self" || sp.str("EffectZone") != "All" {
		return nil, true, refuse("AlternativeCost shape")
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
	if sc.st.Cost.Forage != nil || sc.st.Cost.ExileG != nil && !sc.st.Cost.ExileHand || sc.st.Cost.Energy != 0 {
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

// flashCasterFlag is a CastWithFlash static's Static.StFlags caster bit.
func (s lStatic) flashCasterFlag() kop.StaticFlags {
	if s.AnyCaster {
		return kop.StFlashAnyCaster
	}
	return 0
}
