package kgen

import (
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gainsAbilitiesOf lowers a Continuous static's GainsAbilitiesOf$ (CR
// 613.1f, "has all activated abilities of ..."): the affected object, the
// static's own source (AffectedDefined$ Self arrives as Card.Self), may
// activate each activated ability of every battlefield permanent the filter
// matches at that moment (gorge layers.go gainedFacesForSpec,
// legal_granted.go grantedAbilitiesLimited). The filter is read relative to
// the source (Other excludes it) and lands in kop.Static.GainsOf; the zone
// stays kop.Static.Zone's ZoneNone (the battlefield, Forge's default). The
// kernel's offer walk is dirac/static_gainsof.go. Other zones, another
// recipient, and artifact or land sources (whose mana abilities the walk
// does not model) are refused by name.
func (lc *lowerCtx) gainsAbilitiesOf(sp *params, st *lStatic, aff string) error {
	spec, ok := sp.get("GainsAbilitiesOf")
	if !ok {
		return nil
	}
	z, hasZone := sp.get("GainsAbilitiesOfZones")
	offField := hasZone && z != "Battlefield"
	f, err := lc.filter(spec)
	if err != nil {
		// A zone this lowering does not walk outranks what its filter reads:
		// Myr Welder's Card.IsImprinted is an Exile filter, so the card is
		// held back by its zone, not by the Imprinted reader.
		if offField {
			return refuse("GainsAbilitiesOfZones$ %q", z)
		}
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("GainsAbilitiesOf$ %q: artifact or land sources", spec)
	}
	for _, c := range f.Clauses {
		if c.Types == 0 || c.Types&(tArtifact|tLand) != 0 {
			return refuse("GainsAbilitiesOf$ %q: artifact or land sources", spec)
		}
	}
	if offField {
		return refuse("GainsAbilitiesOfZones$ %q", z)
	}
	if aff != "Card.Self" {
		return refuse("GainsAbilitiesOf Affected %q", aff)
	}
	st.GainsOf = f
	return nil
}

// battlefieldFilterPair builds a Continuous static's two battlefield filter
// fields, Landwalk and GainsOf (cardBuilder.static's filter closure).
func battlefieldFilterPair(filter func(*lFilter, kop.Zone) (kop.FilterID, error), landwalk, gainsOf *lFilter) (kop.FilterID, kop.FilterID, error) {
	lw, err := filter(landwalk, kop.ZoneBattlefield)
	if err != nil {
		return 0, 0, err
	}
	g, err := filter(gainsOf, kop.ZoneBattlefield)
	return lw, g, err
}

// dumpStaticFilters prints a static's Landwalk and GainsOf filters in the IR
// dump (dumpLIR).
func dumpStaticFilters(w io.Writer, s *lStatic) {
	if s.Landwalk != nil {
		fmt.Fprintf(w, " landwalk=%s", s.Landwalk)
	}
	if s.GainsOf != nil {
		fmt.Fprintf(w, " gainsof=%s", s.GainsOf)
	}
}

// staticFilterLabels are the OPS.md labels of a table static's filter fields
// (opsUsage).
func staticFilterLabels(st *kop.Static) []string {
	var out []string
	for _, f := range []struct {
		id    kop.FilterID
		label string
	}{{st.ProtFrom, "st:ProtFrom"}, {st.ProtExc, "st:ProtExc"}, {st.Landwalk, "st:Landwalk"}, {st.GainsOf, "st:GainsOf"}} {
		if f.id != 0 {
			out = append(out, f.label)
		}
	}
	return out
}
