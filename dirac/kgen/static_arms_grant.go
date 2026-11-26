package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go): the
// grants of a keyword or a cast form to cards off the battlefield, and the
// P/T and colour-word markers.
func init() {
	registerStaticBuild("CDAPT", cbCDAPTStatic)
	registerStaticBuild("CantBeBeamedUp", cbCantBeBeamedUpStatic)
	registerStaticBuild("BeamGrant", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatBeamGrant
		return nil, nil
	})
	registerStaticBuild("FbGrant", cbFbGrantStatic)       // kw_fbgrant.go
	registerStaticBuild("WarpGrant", cbWarpGrantStatic)   // kw_warpgrant.go
	registerStaticBuild("MiracleGrant", cbMiracleGrant)   // kw_miracle.go
	registerStaticBuild("MadnessGrant", cbMadnessGrant)   // kw_madness.go
	registerStaticCostBuild("EscapeGrant", cbEscapeGrant) // kw_escape.go
	registerStaticBuild("ColorWords", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatColorWords // static_colorwords.go: no filter, no zone; the stall rule reads the live sources
		return nil, nil
	})
	registerStaticBuild("GraveNoAbilities", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cb.graveNoAbilities(st, s) // lower_offzone_abilityloss.go
	})
	registerStaticBuild("StackKw", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cb.stackKwStatic(st, s) // AffectedZone$ Stack keyword grant (lower_stackkw.go)
	})
}

func cbCDAPTStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCDAPT
	st.StFlags |= kop.StDynamic
	if st.VA, err = cb.value(s.Power); err != nil {
		return nil, err
	}
	st.VB, err = cb.value(s.Tough)
	return nil, err
}

func cbCantBeBeamedUpStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCantBeBeamedUp
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		if fl != "BeamGranted" {
			return nil, fmt.Errorf("CantBeBeamedUp flag %q", fl)
		}
		st.StFlags |= kop.StBeamGranted
	}
	return nil, nil
}

func cbFbGrantStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatFbGrant
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneGraveyard); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		if fl != "OwnCost" {
			return nil, fmt.Errorf("FbGrant flag %q", fl)
		}
		st.StFlags |= kop.StFbOwnCost
		if st.GrantCost, err = manaCostOf(s.Cost.Mana); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func cbWarpGrantStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatWarpGrant
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneHand); err != nil {
		return nil, err
	}
	st.GrantCost, err = manaCostOf(s.Cost.Mana)
	return nil, err
}

func cbMiracleGrant(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatMiracleGrant
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneHand); err != nil {
		return nil, err
	}
	st.Zone = kop.ZoneHand
	st.CardCostLess = uint8(s.CardCostLess)
	st.GrantCost, err = manaCostOf(s.Cost.Mana)
	return nil, err
}

// cbMadnessGrant: an AffectedZone$ <six zones> AddKeyword$ Madness static,
// one static per listed zone (kop.StatMadnessGrant; kw_madness.go; CR
// 702.35a; Falkenrath Gorger). Filter is read in the static's own zone.
func cbMadnessGrant(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatMadnessGrant
	st.Zone = zoneMap[s.Zone]
	if st.Filter, err = cb.staticFilter(s.F, st.Zone); err != nil {
		return nil, err
	}
	st.CardCostLess = uint8(s.CardCostLess)
	st.GrantCost, err = manaCostOf(s.Cost.Mana)
	return nil, err
}

// cbEscapeGrant: an AffectedZone$ Graveyard AddKeyword$ Escape static: each
// matching card in its controller's graveyard has escape at its mana cost
// plus N other cards from the same graveyard (kop.StatEscapeGrant;
// kw_escape.go; CR 702.42a; Underworld Breach). Filter is the affected cards;
// the ability's own Cost is the fodder part, as an AlternativeCost static's
// is: CostExileGrave of N cards matching Cost.Filter (the grant's
// ExileFromGrave spec), a table cost the kernel's cost builder returns in
// place. The mana part is the card's own cost, priced at the offer like any
// spell's.
func cbEscapeGrant(cb *cardBuilder, s lStatic, st *kop.Static, cost *kop.Cost) ([]uint8, error) {
	var err error
	st.Kind = kop.StatEscapeGrant
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneGraveyard); err != nil {
		return nil, err
	}
	if s.Cost.ExileG == nil || s.Cost.ExileGN < 1 || s.Cost.ExileGN > 15 {
		return nil, fmt.Errorf("escape grant without an exile cost")
	}
	if *cost, err = cb.cost(s.Cost); err != nil {
		return nil, err
	}
	if cost.Flags != kop.CostExileGrave || cost.Mana != (kop.ManaCost{}) {
		return nil, fmt.Errorf("escape grant cost beside its exile part")
	}
	return nil, nil
}
