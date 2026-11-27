package kgen

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go): the
// cost, cast and land-play statics.
func init() {
	registerStaticBuild("TapExtra", cbTapExtraStatic)
	registerStaticBuild("CantPlayLand", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cantPlayLandStatic(s, st)
	})
	registerStaticBuild("RaiseCost", cbRaiseCostStatic)
	registerStaticBuild("ManaConvert", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		var err error
		st.Kind, st.Filter, err = cb.manaConvertOp(s) // lower_manaconvert.go
		return nil, err
	})
	registerStaticBuild("CastWithFlash", cbCastWithFlashStatic)
	registerStaticCostBuild("AlternativeCost", cbAlternativeCostStatic)
	registerStaticBuild("CyclingAlt", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCyclingAlt // static_cyclingalt.go: the gate is Cond2, the cost the twin's (build_cyclingalt.go)
		return nil, nil
	})
	registerStaticBuild("ExtraLandPlay", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatExtraLandPlay
		return nil, cb.staticAmounts(st, s.Amount, lValue{})
	})
}

func cbTapExtraStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatTapExtra
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if s.Zone == zCommand {
		st.Zone = kop.ZoneCommand // a Vanguard's command-zone static (Sisay)
	}
	st.A, st.B = int16(s.TapCol), int16(s.TapAmt)
	for _, fl := range s.Flags {
		f, ok := tapExtraFlag(fl)
		if !ok {
			return nil, fmt.Errorf("tap extra flag %q", fl)
		}
		st.StFlags |= f
	}
	if s.Amount.Kind != 0 {
		if st.VA, err = cb.value(s.Amount); err != nil {
			return nil, err
		}
		st.StFlags |= kop.StTapDynamic
	}
	return nil, nil
}

func cbRaiseCostStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatIncreaseCost
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneStack); err != nil {
		return nil, err
	}
	if err := cb.staticTargets2(st, s.F2); err != nil {
		return nil, err
	}
	if err := cb.staticAmounts(st, s.Amount, lValue{}); err != nil {
		return nil, err
	}
	if err := raiseCostStFlags(s, st); err != nil {
		return nil, err
	}
	if s.Zone == zAny {
		st.Zone = kop.ZoneCommand
	} else if s.Zone == zStack {
		st.Zone = kop.ZoneStack // the spell's own stack-zone raise (lower_raisecost.go)
	}
	return nil, nil
}

func cbCastWithFlashStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCastWithFlash
	st.StFlags |= s.flashCasterFlag()
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneHand); err != nil {
		return nil, err
	}
	if slices.Contains(s.Flags, "IsTargeting") {
		// a self grant's Spell.IsTargeting Valid <spec> (lower_static.go): read
		// over the battlefield candidates of the spell's targets
		st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield)
		return nil, err
	}
	st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneStack) // an emblem's ForgetOnCast$ (lower_effectemblem.go)
	return nil, err
}

func cbAlternativeCostStatic(cb *cardBuilder, s lStatic, st *kop.Static, cost *kop.Cost) ([]uint8, error) {
	var err error
	st.Kind = kop.StatAlternativeCost
	if *cost, err = cb.altCostStatic(s, st); err != nil { // static_cost.go: a grant's flag and filter too
		return nil, err
	}
	// the zone the static is offered from (kernel prepAltFree's one
	// home of the hand walk's exclusion, offerAltFreeHand, and the
	// graveyard walk's arm, offerGraveyardAltCasts): 0 (battlefield)
	// for the EffectZone$ All and EffectZone$-less statics, which
	// read no zone today; ZoneGraveyard for the explicit-Graveyard
	// one
	st.Zone = zoneMap[s.Zone]
	if cost.Flags&(kop.CostForage|kop.CostExileGrave) != 0 && !cb.altHandExileOK(cost) && !altGraveExileShape(cost) {
		if altHandExileShape(cost) {
			// the hand-exile shape lowers (the Force cycle); the card
			// carries no alternative-cost cast (altSacOK: a spell
			// ability with a non-mana cost, an either-or or printed
			// additional cost)
			return nil, fmt.Errorf("alternative cost with an exile-from-hand part on a card the kernel offers no alternative cast for")
		}
		return nil, fmt.Errorf("alternative cost with a forage / exile-from-graveyard part")
	}
	// a sacrifice (alone or with life), a return, a hand exile, a
	// graveyard exile, a discard, life alone or taps: the kernel's
	// altPickCost picks, now with the mana part beside them too (a
	// Borderpost's {1} plus a Return, Sephara's {W} plus taps, Squee's
	// {3}{R}; prepAltFree). A grant's dynamic colour-share discard
	// (Dream Halls) is the one non-mana grant.
	grantDiscard := st.StFlags&kop.StAltGrant != 0 && altGrantDiscardCost(cost)
	sacAlt := altPickShape(cost) && cb.altSacOK(cost)
	if !sacAlt && !grantDiscard && (cost.Flags != 0 || cost.N != 0 || cost.Mana.X != 0 && !s.AltXMana) {
		// the kernel offers a mana-only alternative cost
		// (prepAltFree); a part the pick stages cannot carry would
		// never be offered. A grant's one {X} is the cast card's mana
		// value (s.AltXMana, Kentaro), priced by altXMana.
		return nil, fmt.Errorf("alternative cost with a non-mana part (the kernel offers mana-only alternatives)")
	}
	return nil, nil
}
